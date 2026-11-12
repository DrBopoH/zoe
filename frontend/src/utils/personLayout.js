import { cleanPersonName, cleanAccountName, isMaskedPan } from './formatters.js'
import { bankKeyByMfo, bankKeyByCard, BANK_ORDER, bankLabel } from './banks.js'

/*
 * Модель данных (зеркалит бэкенд):
 *
 *   КЭШ (cache.db)  — то, что видно в банковских данных прямо сейчас (восполняемо).
 *   СТАТИКА (core.db) — лица, привязка алиас→лицо, запомненная структура алиас→IBAN→PAN, теневые баны.
 *
 *   Алиас — единица связи. Структура алиаса = набор счетов (IBAN) и карт (PAN) с указанием «PAN принадлежит IBAN».
 *   Эффективная структура алиаса = (статика ∪ кэш) − теневые баны.   Зелёный = подтверждено кэшем, жёлтый = только статика.
 */

export const normId = (s) => String(s || '').replace(/[^a-zA-Z0-9]/g, '').toUpperCase()
export const isIbanLike = (id) => /^[A-Z]{2}\d{2}/.test(normId(id))

// Ключ идентификатора: IBAN — целиком, PAN — первые 4 + последние 4 (маскированные карты сравниваются по ним)
export const idKey = (s) => {
  const n = normId(s)
  if (!n) return ''
  if (isIbanLike(n)) return n
  return n.length >= 8 ? n.slice(0, 4) + n.slice(-4) : n
}
export const panKey = idKey

// Номер карты (полный или с маской), не IBAN — зеркало isPanLike в Go (util.go)
export const isPanLike = (s) => {
  const n = normId(s)
  return n.length >= 8 && !isIbanLike(n) && /^\d+$/.test(n)
}

// «Слабый» ключ карты: BIN (6 цифр) + последние 2. ПриватБанк показывает получателя как 414949****72,
// а карту — как 414949******0172: по idKey (4+4) они не сойдутся, по BIN+2 — сойдутся. Зеркало looseKey в Go.
export const looseKey = (s) => {
  const str = String(s || '').trim()
  if (!str || isIbanLike(str)) return ''
  const i = str.search(/[*•xX]/)
  const digits = (x) => x.replace(/\D/g, '')
  if (i < 0) {
    const n = str.replace(/[^a-zA-Z0-9]/g, '')
    return n.length >= 12 && /^\d+$/.test(n) ? n.slice(0, 6) + n.slice(-2) : ''
  }
  const lead = digits(str.slice(0, i))
  const tail = digits(str.slice(i))
  return lead.length >= 6 && tail.length >= 2 ? lead.slice(0, 6) + tail.slice(-2) : ''
}

// idKey, но '' для масок с <4 открытыми цифрами в конце (414949****72): их «4+4» ключ — мусор
export const reliableKey = (s) => {
  const str = String(s || '')
  const i = str.search(/[*•xX]/)
  if (i >= 0 && str.slice(i).replace(/\D/g, '').length < 4) return ''
  return idKey(str)
}

// Ключ алиаса: lower(trim)
export const aliasKey = (s) => String(s || '').trim().toLowerCase()

export const pansOf = (acc) => {
  if (!acc) return []
  if (Array.isArray(acc.pans) && acc.pans.length > 0) return acc.pans.filter(Boolean)
  return String(acc.masked_pan || '').split(',').map(s => s.trim()).filter(Boolean)
}

export const detectBankKey = (ibanStr, cacheAcc, pans = []) => {
  const norm = normId(ibanStr)
  const mfo = isIbanLike(norm) ? norm.slice(4, 10) : ''
  const type = (cacheAcc?.type || '').toLowerCase()

  if (cacheAcc?.bank_key === 'icu') return 'icu'
  if (cacheAcc?.bank_key === 'binance') return 'binance'
  const byMfoKey = bankKeyByMfo(mfo)
  if (byMfoKey) return byMfoKey
  if (/^BN\d{6,12}$/.test(norm)) return 'binance' // профиль Binance (BN + id пользователя из выписки)

  const allIds = [norm, ...pans.map(normId)].filter(Boolean)
  for (const id of allIds) {
    const k = bankKeyByCard(id)
    if (k) return k
  }
  if (type) return 'mono'
  return 'other'
}

// ───────────────────────── кто такой «контрагент-человек» ─────────────────────────
// У Privat (PDF) counter_name заполнен и у магазинов (WOG, EKVATOR…), поэтому «есть counter_name» — не признак человека.
// Человек = есть IBAN контрагента, ИЛИ MCC перевода, ИЛИ в описании признак перевода,
// ИЛИ имя есть, а описания нет вовсе (источник дал только имя).
export const TRANSFER_MCC = new Set([4829, 6538, 6536, 6537, 6012, 6011])
const TRANSFER_RE = /(^|[\s,.;:])(переказ|перекази|зарахування\s+переказу|поповнення|від[:\s]|з\s+картки|на\s+картку|p2p)/i

// Контакт из описания («Від: Іван», «Переказ: drbopoh raif»): только если префикс перевода реально был срезан
// и остаток похож на имя, а не на «Перекази, УКРАЇНА, термінал CP980…»
export const contactFromDescription = (desc) => {
  const raw = String(desc || '').trim()
  if (!raw) return ''
  const cleaned = cleanPersonName(raw)
  if (!cleaned || cleaned === raw) return ''
  if (cleaned.length > 60 || /[,;]/.test(cleaned)) return ''
  return cleaned
}

// Mono: у P2P-перевода (MCC 4829) описание — это просто имя («Нуся ❤️»), без префикса «Від:».
// Берём его как имя только для «чистых» переводов: короткое, без запятых/цифр-масок и без слов-признаков перевода.
const PLAIN_NAME_MCC = new Set([4829, 6538, 6536, 6537])
export const plainNameFromDescription = (t) => {
  if (!PLAIN_NAME_MCC.has(Number(t?.mcc))) return ''
  const raw = String(t.description || '').trim()
  if (!raw || raw.length > 40 || /[,;:]/.test(raw)) return ''
  if (TRANSFER_RE.test(raw) || /^[\d\*\s]+$/.test(raw)) return ''
  return raw
}

// Единое «имя контрагента» операции: counter_name → контакт после префикса → имя-описание P2P
// Контрагент без имени, известный только по карте («отримувач -, 487407****44, банк отримувача JSC UNIVERSAL BANK»):
// имя-алиас «487407****44 · Monobank». Зеркало pseudoCounterName в Go (util.go).
export const pseudoCounterName = (t) => {
  const c = String(t?.counter_iban || '').trim()
  if (!c || !isPanLike(c)) return ''
  let bank = bankLabel(detectBankKey('', null, [c]))
  if (!bank) {
    const m = /банк\s+(?:отримувача|платника)\s+(.+?)(?:,|$)/i.exec(String(t.description || ''))
    bank = m ? m[1].trim() : ''
    if (bank === '-') bank = ''
  }
  return bank ? `${c} · ${bank}` : c
}

// Единое «имя контрагента» операции: counter_name → контакт после префикса → имя-описание P2P → карта · банк
export const counterpartyName = (t) =>
  cleanPersonName(t?.counter_name) || contactFromDescription(t?.description) || plainNameFromDescription(t) || pseudoCounterName(t)

export const isPersonTransaction = (t) => {
  if (!t) return false
  if (isIbanLike(t.counter_iban) || isPanLike(t.counter_iban)) return true
  if (TRANSFER_MCC.has(Number(t.mcc))) return true
  const desc = String(t.description || '')
  if (TRANSFER_RE.test(desc)) return true
  return !!String(t.counter_name || '').trim() && desc.trim() === ''
}

export const isPartialPan = (s) => isPanLike(s) && reliableKey(s) === '' && looseKey(s) !== ''

// Один и тот же идентификатор в пределах одного алиаса: совпал idKey, либо одна из карт «частичная» с тем же BIN+2.
export const sameId = (a, b) => {
  if (idKey(a) === idKey(b)) return true
  const la = looseKey(a)
  return la !== '' && la === looseKey(b) && (isPartialPan(a) || isPartialPan(b))
}

// ───────────────────────── индекс кэша ─────────────────────────
// Правила те же, что в Go (resolve.go):
//  • владелец счёта: cleanAccountName(account.name) → IBAN + все PAN счёта
//  • контрагент (P2P): cleanPersonName(counter_name), иначе контакт из описания → counter_iban (+ PAN, если это наш счёт)
//  • безымянный контрагент с IBAN → псевдо-алиас, названный самим IBAN (если IBAN не наш)

export const buildCacheIndex = (accounts = [], transactions = []) => {
  const byAlias = new Map()        // aliasKey → { key, name, links: Map(idKey → {id,parent}), txCount }
  const ibanByPan = new Map()      // idKey(PAN) → IBAN (строкой из кэша)
  const accountsByIban = new Map() // normIBAN → account
  const cacheIds = new Set()       // все idKey, которые видит кэш

  const ensure = (name) => {
    const key = aliasKey(name)
    if (!key) return null
    let n = byAlias.get(key)
    if (!n) {
      n = { key, name: String(name).trim(), links: new Map(), txCount: 0 }
      byAlias.set(key, n)
    }
    return n
  }
  const addLink = (node, id, parent) => {
    const k = idKey(id)
    if (!k) return
    cacheIds.add(k)
    for (const [ek, ex] of node.links) {
      if (!sameId(ex.id, id)) continue
      // та же карта: полный номер вытесняет частичный (414949****72 → 414949******0172)
      if (isPartialPan(ex.id) && !isPartialPan(id)) {
        node.links.delete(ek)
        node.links.set(k, { id: String(id).trim(), parent: parent || ex.parent || '' })
      } else if (parent && !ex.parent) ex.parent = parent
      return
    }
    node.links.set(k, { id: String(id).trim(), parent: parent || '' })
  }

  const ownByPanKey = new Map()    // reliableKey(PAN) → наш счёт
  const ownByLoose = new Map()     // looseKey(PAN) → [наши счета]
  ;(accounts || []).forEach(acc => {
    pansOf(acc).forEach(p => {
      const rk = reliableKey(p)
      if (rk) ownByPanKey.set(rk, acc)
      const lk = looseKey(p)
      if (lk) ownByLoose.set(lk, [...(ownByLoose.get(lk) || []), acc])
    })
  })
  // чей это счёт по номеру карты контрагента: точно по ключу, иначе по BIN+2 — только если счёт один
  const ownByPan = (pan) => {
    const rk = reliableKey(pan)
    if (rk && ownByPanKey.has(rk)) return ownByPanKey.get(rk)
    const lk = looseKey(pan)
    const list = lk ? ownByLoose.get(lk) : null
    return list && list.length === 1 ? list[0] : null
  }

  ;(accounts || []).forEach(acc => {
    const ibanOk = acc.iban && isIbanLike(acc.iban)
    const iban = ibanOk ? String(acc.iban).trim() : ''
    const pans = pansOf(acc)

    if (iban) {
      accountsByIban.set(normId(iban), acc)
      cacheIds.add(idKey(iban))
    }
    pans.forEach(p => {
      cacheIds.add(idKey(p))
      if (iban) ibanByPan.set(idKey(p), iban)
    })

    const holder = ensure(cleanAccountName(acc.name)) // счёт из файла (ICU, Binance) — обычный счёт: алиас по имени файла
    if (holder) {
      if (iban) addLink(holder, iban, '')
      pans.forEach(p => addLink(holder, p, iban))
    }
  })

  ;(transactions || []).forEach(t => {
    if (!isPersonTransaction(t)) return
    const cRaw = String(t.counter_iban || '').trim()
    const cIban = isIbanLike(cRaw) ? cRaw : ''
    const cPan = !cIban && isPanLike(cRaw) ? cRaw : ''
    const own = cIban ? accountsByIban.get(normId(cIban)) : (cPan ? ownByPan(cPan) : null)
    const ownIban = own && own.iban && isIbanLike(own.iban) ? String(own.iban).trim() : ''

    let name = counterpartyName(t)
    if (!name && cIban && !own) name = cIban // псевдо-алиас
    if (!name) return

    const node = ensure(name)
    node.txCount += 1
    if (cIban) {
      addLink(node, cIban, '')
      if (own) pansOf(own).forEach(p => addLink(node, p, cIban))
    } else if (cPan) {
      if (ownIban) {
        // карта наша → тянем весь счёт: IBAN и все его PAN
        addLink(node, ownIban, '')
        pansOf(own).forEach(p => addLink(node, p, ownIban))
      } else {
        addLink(node, cPan, '')
      }
    }
  })

  return {
    byAlias,
    ibanByPan,
    accountsByIban,
    cacheIds,
    aliasKeys: new Set(byAlias.keys())
  }
}

// ───────────────────────── граф алиасов ─────────────────────────

const trim = (s) => String(s || '').trim()

export const structuredAccounts = (node) =>
  (node?.effective || []).map(e => ({
    account_identifier: e.id,
    parent_identifier: e.parent,
    cached: e.cached,
    persisted: e.persisted
  }))

export const buildAliasGraph = ({ accounts = [], transactions = [], users = [], structures = [] } = {}) => {
  const cache = buildCacheIndex(accounts, transactions)
  const nodes = new Map()

  const ensure = (name) => {
    const key = aliasKey(name)
    if (!key) return null
    let n = nodes.get(key)
    if (!n) {
      n = { key, name: trim(name), userId: '', raw: new Map(), bans: new Set(), txCount: 0, inCache: false, effective: [] }
      nodes.set(key, n)
    }
    return n
  }
  const addRaw = (n, id, parent, flags) => {
    const k = idKey(id)
    if (!n || !k) return
    let e = n.raw.get(k)
    if (!e) {
      e = { key: k, id: trim(id), parent: '', cached: false, persisted: false }
      n.raw.set(k, e)
    }
    // та же карта в маске и настоящим номером — оставляем чистое число
    if (isMaskedPan(e.id) && !isMaskedPan(id)) e.id = trim(id)
    if (parent && !e.parent) e.parent = trim(parent)
    if (flags.cached) e.cached = true
    if (flags.persisted) e.persisted = true
  }

  // 1. статика: структуры алиасов (включая бесхозные)
  ;(structures || []).forEach(s => {
    const n = ensure(s.alias)
    if (!n) return
    if (s.user_id) n.userId = s.user_id
    ;(s.accounts || []).forEach(l => addRaw(n, l.account_identifier, l.parent_identifier, { persisted: true, cached: !!l.confirmed }))
  })

  // 2. статика: лица (источник истины по привязке) и теневые баны
  ;(users || []).forEach(u => {
    if (!u || u.isDraft) return
    ;(u.aliases || []).forEach(a => {
      const n = ensure(a)
      if (n) n.userId = u.id
    })
    ;(u.alias_links || []).forEach(l => {
      const n = ensure(l.alias)
      addRaw(n, l.account_identifier, l.parent_identifier, { persisted: true, cached: !!l.confirmed })
    })
    ;(u.alias_bans || []).forEach(b => {
      const n = ensure(b.alias)
      if (n) n.bans.add(idKey(b.account_identifier))
    })
  })

  // 3. кэш
  cache.byAlias.forEach(c => {
    const n = ensure(c.name)
    n.inCache = true
    n.txCount = c.txCount
    c.links.forEach(l => addRaw(n, l.id, l.parent, { cached: true }))
  })

  // 3б. частичная карта (414949****48) из статики и полный номер (414949******1748) — одна карта:
  //     частичная растворяется в полной, если полная в алиасе ровно одна (как в addLink для кэша)
  nodes.forEach(n => {
    const all = [...n.raw.values()]
    all.filter(e => isPartialPan(e.id)).forEach(p => {
      const fulls = all.filter(f => f !== p && !isPartialPan(f.id) && !isIbanLike(f.id) && sameId(p.id, f.id))
      if (fulls.length !== 1) return
      const f = fulls[0]
      f.persisted = f.persisted || p.persisted
      f.cached = f.cached || p.cached
      if (!f.parent && p.parent) f.parent = p.parent
      n.raw.delete(p.key)
    })
  })

  // 4. эффективная структура: родитель PAN из кэша → из статики; минус баны
  nodes.forEach(n => {
    const cn = cache.byAlias.get(n.key)
    n.raw.forEach(e => {
      if (cn && cn.links.has(e.key)) e.cached = true
      if (cache.cacheIds.has(e.key)) e.cached = true
    })

    const list = []
    n.raw.forEach(e => {
      let parent = ''
      if (!isIbanLike(e.id)) {
        parent = cache.ibanByPan.get(e.key) || e.parent || ''
        const same = parent ? n.raw.get(idKey(parent)) : null
        if (same) parent = same.id
      }
      if (n.bans.has(e.key)) return
      if (parent && n.bans.has(idKey(parent))) return
      list.push({ key: e.key, id: e.id, parent, cached: e.cached, persisted: e.persisted })
    })
    // IBAN-ы первыми, затем карты — стабильный порядок
    list.sort((a, b) => Number(!isIbanLike(a.id)) - Number(!isIbanLike(b.id)))
    n.effective = list
  })

  const isCached = (sig) => {
    if (!sig) return false
    const ak = aliasKey(sig)
    if (ak && cache.aliasKeys.has(ak)) return true
    const ik = idKey(sig)
    return !!ik && cache.cacheIds.has(ik)
  }

  const orphans = Array.from(nodes.values())
    // алиас файла из кэша виден в бесхозе и без привязок: IBAN брокера ему ставят руками
    .filter(n => !n.userId && (n.effective.length > 0 || n.txCount > 0 || n.inCache))

  return { nodes, cache, isCached, orphans, nodeFor: (alias) => nodes.get(aliasKey(alias)) || null }
}

// ───────────────────────── раскладка карточки лица ─────────────────────────

const byName = (a, b) => String(a).localeCompare(String(b), 'ru')

export const buildPersonLayout = (user, graph) => {
  const empty = { looseAliases: [], groups: [] }
  if (!user || !graph) return empty

  const cache = graph.cache

  // уникальные алиасы лица
  const aliasList = []
  const seenAlias = new Set()
  ;(user.aliases || []).forEach(a => {
    const k = aliasKey(a)
    if (k && !seenAlias.has(k)) {
      seenAlias.add(k)
      aliasList.push({ alias: trim(a), key: k, node: graph.nodeFor(a) })
    }
  })

  // собираем все идентификаторы лица
  const entries = new Map() // idKey → { key, id, parent, aliasKeys:Set }
  aliasList.forEach(({ key, node }) => {
    ;(node?.effective || []).forEach(e => {
      let en = entries.get(e.key)
      if (!en) {
        en = { key: e.key, id: e.id, parent: '', aliasKeys: new Set() }
        entries.set(e.key, en)
      }
      if (e.parent && !en.parent) en.parent = e.parent
      en.aliasKeys.add(key)
    })
  })

  // плитки по IBAN
  const tiles = []
  const tileByKey = new Map()
  entries.forEach(en => {
    if (!isIbanLike(en.id)) return
    const norm = normId(en.id)
    const acc = cache.accountsByIban.get(norm) || null
    const tile = { key: norm, iban: en.id, norm, pans: [], aliasKeys: en.aliasKeys, cacheAcc: acc, type: acc?.type || '', masterKey: '' }
    tiles.push(tile)
    tileByKey.set(norm, tile)
  })

  // IBAN-плитки каждого алиаса (для карт, у которых родитель неизвестен)
  const ibanTilesOfAlias = new Map() // aliasKey → Set(tile)
  tiles.forEach(tile => tile.aliasKeys.forEach(ak => {
    if (!ibanTilesOfAlias.has(ak)) ibanTilesOfAlias.set(ak, new Set())
    ibanTilesOfAlias.get(ak).add(tile)
  }))

  // карты → плитка:
  //  1) кэш знает, к какому IBAN карта относится (реальная топология, перевыпуск и т.п.);
  //  2) статика помнит родителя;
  //  3) родитель неизвестен нигде, а у владельцев карты ровно один IBAN → карта его;
  //  4) иначе карта без IBAN — своя отдельная плитка (по одной на PAN) в банке владельцев,
  //     а не общая куча «карт без счёта» в банке, угаданном по первой цифре.
  const tileOfPan = new Map()
  entries.forEach(en => {
    if (isIbanLike(en.id)) return
    const cacheParent = cache.ibanByPan.get(en.key)
    let tile = cacheParent ? tileByKey.get(normId(cacheParent)) : null
    if (!tile && !cacheParent && en.parent) tile = tileByKey.get(normId(en.parent))

    const candidates = new Set()
    en.aliasKeys.forEach(ak => (ibanTilesOfAlias.get(ak) || new Set()).forEach(c => candidates.add(c)))

    if (!tile && !cacheParent && candidates.size === 1) tile = [...candidates][0]

    if (!tile) {
      const banks = new Set([...candidates].map(c => detectBankKey(c.iban, c.cacheAcc, c.pans)))
      const bank = banks.size === 1 ? [...banks][0] : detectBankKey(en.id)
      tile = { key: 'pan:' + en.key, iban: null, norm: '', pans: [], aliasKeys: new Set(), cacheAcc: null, type: '', masterKey: '', forcedBank: bank }
      tiles.push(tile)
    }
    tile.pans.push(en.id)
    tileOfPan.set(en.key, tile)
  })

  // главный PAN, порядок карт, банк, главный алиас плитки
  tiles.forEach(t => {
    const cachePans = t.cacheAcc ? pansOf(t.cacheAcc) : []
    const order = (p) => {
      const i = cachePans.findIndex(cp => idKey(cp) === idKey(p))
      return i === -1 ? 999 : i
    }
    t.pans.sort((a, b) => order(a) - order(b))

    const cacheMainKey = cachePans.length > 0 ? idKey(cachePans[0]) : ''
    t.mainPan = t.pans.find(p => idKey(p) === cacheMainKey) || t.pans[0] || null
    const mk = t.mainPan ? idKey(t.mainPan) : ''
    t.otherPans = t.pans.filter(p => idKey(p) !== mk)
    t.bankKey = t.forcedBank || detectBankKey(t.iban || t.pans[0], t.cacheAcc, t.pans)

    if (t.iban) {
      const holder = t.cacheAcc ? aliasKey(cleanAccountName(t.cacheAcc.name)) : ''
      const keys = [...t.aliasKeys]
      if (holder && t.aliasKeys.has(holder)) t.masterKey = holder
      else t.masterKey = keys.map(k => ({ k, name: seenName(aliasList, k) })).sort((a, b) => byName(a.name, b.name))[0]?.k || ''
    }
  })

  // алиасы: без структуры → «загулявшие»; иначе в группы банков, где у них есть плитки
  const looseAliases = []
  const bankAliases = new Map() // bankKey → Map(aliasKey → item)

  aliasList.forEach(({ alias, key, node }) => {
    const eff = node?.effective || []
    if (eff.length === 0) {
      looseAliases.push({ alias, accounts: [] })
      return
    }
    const mine = new Set()
    eff.forEach(e => {
      const t = isIbanLike(e.id) ? tileByKey.get(normId(e.id)) : tileOfPan.get(e.key)
      if (t) mine.add(t)
    })
    const banks = new Set([...mine].map(t => t.bankKey))
    banks.forEach(bk => {
      const ibanTiles = [...mine].filter(t => t.iban && t.bankKey === bk)
      const isSub = ibanTiles.length > 0 && !ibanTiles.some(t => t.masterKey === key)
      if (!bankAliases.has(bk)) bankAliases.set(bk, new Map())
      bankAliases.get(bk).set(key, { alias, accounts: structuredAccounts(node), isSub })
    })
  })

  const groupsMap = new Map()
  const group = (bk) => {
    if (!groupsMap.has(bk)) groupsMap.set(bk, { key: bk, aliases: [], tiles: [] })
    return groupsMap.get(bk)
  }
  tiles.forEach(t => group(t.bankKey).tiles.push(t))
  bankAliases.forEach((m, bk) => {
    group(bk).aliases = [...m.values()].sort((a, b) => (Number(a.isSub) - Number(b.isSub)) || byName(a.alias, b.alias))
  })

  const groups = []
  BANK_ORDER.forEach(bk => { if (groupsMap.has(bk)) { groups.push(groupsMap.get(bk)); groupsMap.delete(bk) } })
  groupsMap.forEach(g => groups.push(g))

  return { looseAliases, groups }
}

function seenName(aliasList, key) {
  const a = aliasList.find(x => x.key === key)
  return a ? a.alias : key
}
