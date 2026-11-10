import { ref } from 'vue'
import { idKey } from '../utils/personLayout.js'
import { apiSend, q } from '../utils/api.js'

export function useLinkEditor(users, emit, fetchUsers) {
  const isModalOpen = ref(false)
  const modalAliasData = ref(null)

  // accounts: [{ account_identifier, parent_identifier, persisted, cached }]
  const openLinkEditor = (aliasName, currentUserId = '', accounts = []) => {
    const list = []
    ;(Array.isArray(accounts) ? accounts : []).forEach(item => {
      if (!item) return
      if (typeof item === 'string') {
        const id = item.trim()
        if (id) list.push({ account_identifier: id, parent_identifier: '', persisted: false, cached: false })
        return
      }
      const id = String(item.account_identifier || '').trim()
      if (!id) return
      list.push({
        account_identifier: id,
        parent_identifier: String(item.parent_identifier || '').trim(),
        persisted: !!item.persisted,
        cached: !!item.cached
      })
    })

    modalAliasData.value = { name: aliasName, user_id: currentUserId || '', accounts: list }
    isModalOpen.value = true
  }

  const closeLinkEditor = () => {
    isModalOpen.value = false
    modalAliasData.value = null
  }

  const quickCreateUser = async (suggestedName = '', callback) => {
    try {
      const name = (suggestedName || 'Новое лицо').trim()
      const res = await apiSend('POST', '/users', { name, avatar_url: '' })
      const created = await res.json()
      await fetchUsers()
      if (emit) emit('refresh')
      if (created?.id && callback) callback(created.id)
    } catch (err) {
      console.error('Quick create user error:', err)
    }
  }

  /*
   * Сохранение. Порядок важен для теневых банов:
   *   1. привязать алиас к лицу (смена владельца гасит старые баны);
   *   2. убрать удалённые счета — пока алиас привязан и кэш подтверждает связь, бэкенд поставит теневой бан;
   *   3. дописать добавленные/ещё не сохранённые в статике (ручное добавление снимает бан);
   *   4. если лицо сняли — отвязать алиас (он уйдёт в бесхоз вместе со структурой).
   */
  const saveLinkEditorData = async ({ alias, user_id, accounts, initialAccounts }) => {
    if (!alias) return
    const initialUserId = modalAliasData.value?.user_id || ''
    // уже сохранено в статике именно в таком написании (маска → чистое число считается изменением)
    const knownPersisted = new Map(
      (modalAliasData.value?.accounts || []).map(a => [idKey(a.account_identifier), a.persisted ? a.account_identifier : null])
    )

    const nextAccounts = (accounts || []).filter(a => a?.account_identifier)
    const nextKeys = new Set(nextAccounts.map(a => idKey(a.account_identifier)))
    const prev = (initialAccounts || []).filter(a => a?.account_identifier)

    const removed = prev.filter(a => !nextKeys.has(idKey(a.account_identifier)))
    const removedKeys = new Set(removed.map(a => idKey(a.account_identifier)))

    try {
      if (user_id) {
        await apiSend('POST', '/users/alias', { user_id, alias })
      }

      for (const r of removed) {
        // карты удалённого IBAN бэкенд убирает каскадом
        if (r.parent_identifier && removedKeys.has(idKey(r.parent_identifier))) continue
        await apiSend('DELETE', `/users/link-account?identifier=${q(r.account_identifier)}&alias=${q(alias)}`)
      }

      for (const acc of nextAccounts) {
        const k = idKey(acc.account_identifier)
        if (knownPersisted.get(k) === acc.account_identifier) continue
        await apiSend('POST', '/users/link-account', {
          user_id: user_id || '',
          account_identifier: acc.account_identifier,
          parent_identifier: acc.parent_identifier || '',
          note: alias
        })
      }

      if (!user_id && initialUserId) {
        await apiSend('DELETE', `/users/alias?alias=${q(alias)}`)
      }
    } catch (err) {
      console.error('Save link editor data error:', err)
    } finally {
      closeLinkEditor()
      await fetchUsers()
      if (emit) emit('refresh')
    }
  }

  return {
    isModalOpen,
    modalAliasData,
    openLinkEditor,
    closeLinkEditor,
    quickCreateUser,
    saveLinkEditorData
  }
}
