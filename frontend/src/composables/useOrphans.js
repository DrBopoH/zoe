import { ref, computed } from 'vue'
import { formatMaskedIBAN } from '../utils/formatters.js'
import { structuredAccounts } from '../utils/personLayout.js'
import { apiSend, q } from '../utils/api.js'

/*
 * «Бесхоз» = алиасы без лица. Всё берётся из графа (кэш ∪ статика), своего хранилища
 * в localStorage больше нет: структуру отвязанного алиаса помнит статика на бэкенде.
 */
export function useOrphans(props, users, emit, fetchUsers, graph) {
  const searchPersonQuery = ref('')
  const draggedItem = ref(null)
  const activeSelectIndex = ref(null)
  const isGhostHovered = ref(false)

  // Алиасы, которые прямо сейчас привязываются (оптимистично прячем из списка)
  const optimisticLinkedIds = ref(new Set())

  const isSignatureInCache = (sig) => graph.value.isCached(sig)

  const orphanSignatures = computed(() => {
    let list = graph.value.orphans
      .filter(n => !optimisticLinkedIds.value.has(n.key))
      .map(n => {
        const accounts = structuredAccounts(n)
        const ids = accounts.map(a => a.account_identifier)
        return {
          id: n.key,
          name: n.name,
          rawAlias: n.name,
          accounts,
          ibansArray: ids,
          ibanDisplay: ids.length > 0 ? ids.map(formatMaskedIBAN).join(' · ') : '—',
          txCount: n.txCount
        }
      })

    const query = searchPersonQuery.value.trim().toLowerCase()
    if (query) {
      list = list.filter(i =>
        i.name.toLowerCase().includes(query) ||
        i.ibanDisplay.toLowerCase().includes(query) ||
        i.ibansArray.some(id => id.toLowerCase().includes(query))
      )
    }
    return list.sort((a, b) => (b.txCount - a.txCount) || a.name.localeCompare(b.name, 'ru'))
  })

  // Привязать алиас к лицу: бэкенд сам запишет в статику структуру, подтверждённую кэшем.
  // Явно досылаем только то, чего кэш не подтверждает (структура из статики, безымянный IBAN).
  const linkSignatureToUser = async (user, item) => {
    if (!user || !item) return
    const alias = item.rawAlias || item.name
    if (!alias) return
    try {
      await apiSend('POST', '/users/alias', { user_id: user.id, alias })
      for (const acc of item.accounts || []) {
        if (acc.persisted) continue
        await apiSend('POST', '/users/link-account', {
          user_id: user.id,
          account_identifier: acc.account_identifier,
          parent_identifier: acc.parent_identifier || '',
          note: alias
        })
      }
      activeSelectIndex.value = null
    } catch (err) {
      console.error('Link signature error:', err)
    } finally {
      await fetchUsers()
      if (emit) emit('refresh')
    }
  }

  // Отвязать алиас от лица → он уходит в бесхоз, его структура остаётся в статике
  const unlinkAlias = async (user, alias) => {
    try {
      await apiSend('DELETE', `/users/alias?alias=${q(alias)}`)
    } catch (err) {
      console.error('Unlink alias error:', err)
    } finally {
      await fetchUsers()
      if (emit) emit('refresh')
    }
  }

  return {
    searchPersonQuery,
    draggedItem,
    activeSelectIndex,
    isGhostHovered,
    optimisticLinkedIds,
    isSignatureInCache,
    orphanSignatures,
    linkSignatureToUser,
    unlinkAlias
  }
}
