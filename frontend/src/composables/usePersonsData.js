import { ref, computed } from 'vue'
import { apiSend, apiGetJson, q } from '../utils/api.js'
import { loadJSON, saveJSON } from '../utils/storage.js'
const STORAGE_CARD_ORDER = 'persons_card_order'

export function usePersonsData(emit) {
  const users = ref([])
  const structures = ref([]) // запомненные структуры алиасов (в т.ч. бесхозных) из статики
  const cardOrder = ref(loadJSON(STORAGE_CARD_ORDER, [])) // порядок карточек переживает перезагрузку

  // Drag & Drop карточек Лиц
  const draggedCardIndex = ref(null)
  const activeHoverUser = ref(null)

  // Редактирование / создание Лица
  const editingUserId = ref(null)
  const editForm = ref({ name: '', avatar_url: '' })
  const initialEditForm = ref({ name: '', avatar_url: '' })

  // Таймеры появления кнопки хот-вставки имени
  const activeNamePasteUsers = ref(new Set())
  const namePasteTimers = ref({})

  const saveCardOrder = () => saveJSON(STORAGE_CARD_ORDER, cardOrder.value)

  const applyCardOrder = (rawUsers) => {
    const sortedRaw = [...rawUsers].sort((a, b) => {
      const idA = String(a.id)
      const idB = String(b.id)
      return idA.localeCompare(idB, undefined, { numeric: true })
    })

    if (cardOrder.value.length === 0) {
      cardOrder.value = sortedRaw.map(u => u.id)
      saveCardOrder()
      return sortedRaw
    }

    const map = new Map(sortedRaw.map(u => [u.id, u]))
    const ordered = []

    cardOrder.value.forEach(id => {
      if (map.has(id)) {
        ordered.push(map.get(id))
        map.delete(id)
      }
    })

    const remaining = Array.from(map.values())
    remaining.forEach(u => {
      ordered.push(u)
      cardOrder.value.push(u.id)
    })

    saveCardOrder()
    return ordered
  }

  const fetchUsers = async () => {
    try {
      const [rawUsers, rawStructures] = await Promise.all([
        apiGetJson('/users'),
        apiGetJson('/alias-structures')
      ])
      structures.value = rawStructures || []
      users.value = applyCardOrder(rawUsers || [])
    } catch (err) {
      console.error('Fetch users error:', err)
    }
  }

  const hasChanges = computed(() => {
    return editForm.value.name !== initialEditForm.value.name ||
           editForm.value.avatar_url !== initialEditForm.value.avatar_url
  })

  const canSave = (user) => {
    if (!editForm.value.name.trim()) return false
    if (user.isDraft) return true
    return hasChanges.value
  }

  const onNameMouseEnter = (userId) => {
    if (namePasteTimers.value[userId]) {
      clearTimeout(namePasteTimers.value[userId])
      delete namePasteTimers.value[userId]
    }
    activeNamePasteUsers.value.add(userId)
  }

  const onNameMouseLeave = (userId) => {
    if (namePasteTimers.value[userId]) {
      clearTimeout(namePasteTimers.value[userId])
    }
    namePasteTimers.value[userId] = setTimeout(() => {
      activeNamePasteUsers.value.delete(userId)
      delete namePasteTimers.value[userId]
    }, 3000)
  }

  const onUserCardMouseLeave = (userId) => {
    if (namePasteTimers.value[userId]) {
      clearTimeout(namePasteTimers.value[userId])
      delete namePasteTimers.value[userId]
    }
    activeNamePasteUsers.value.delete(userId)
  }

  const startCreateUser = (droppedItem = null) => {
    cancelDrafts()

    const tempId = 'draft_' + Date.now()
    const draftUser = {
      id: tempId,
      name: '',
      avatar_url: '',
      aliases: droppedItem?.rawAlias ? [droppedItem.rawAlias] : [],
      ibans: droppedItem?.ibansArray ? [...droppedItem.ibansArray] : [],
      isDraft: true,
      droppedItem: droppedItem || null
    }

    users.value.push(draftUser)
    editingUserId.value = tempId
    editForm.value.name = ''
    editForm.value.avatar_url = ''
    initialEditForm.value = { name: '', avatar_url: '' }
    return draftUser
  }

  const cancelDrafts = (onCancelDraft) => {
    users.value = users.value.filter(u => {
      if (u.isDraft) {
        if (u.droppedItem && onCancelDraft) {
          onCancelDraft(u.droppedItem.id)
        }
        return false
      }
      return true
    })
  }

  const startEditUser = (user) => {
    cancelDrafts()
    editingUserId.value = user.id
    editForm.value.name = user.name || ''
    editForm.value.avatar_url = user.avatar_url || ''
    initialEditForm.value = { name: editForm.value.name, avatar_url: editForm.value.avatar_url }
  }

  const cancelEdit = (user, onCancelDraft) => {
    if (user.isDraft) {
      if (user.droppedItem && onCancelDraft) {
        onCancelDraft(user.droppedItem.id)
      }
      users.value = users.value.filter(u => u.id !== user.id)
    }
    editingUserId.value = null
    editForm.value.name = ''
    editForm.value.avatar_url = ''
    initialEditForm.value = { name: '', avatar_url: '' }
  }

  const saveUser = async (user, onSaveDraftSuccess) => {
    if (!canSave(user)) return

    const name = editForm.value.name.trim()
    const avatar_url = editForm.value.avatar_url.trim()

    try {
      if (user.isDraft) {
        const res = await apiSend('POST', '/users', { name, avatar_url })
        const created = await res.json()
        await fetchUsers()
        if (user.droppedItem && onSaveDraftSuccess && created?.id) {
          await onSaveDraftSuccess(created, user.droppedItem)
        }
      } else {
        await apiSend('PUT', '/users', { id: user.id, name, avatar_url })
        await fetchUsers()
      }

      editingUserId.value = null
      editForm.value.name = ''
      editForm.value.avatar_url = ''
      initialEditForm.value = { name: '', avatar_url: '' }
      if (emit) emit('refresh')
    } catch (err) {
      console.error('Save user error:', err)
    }
  }

  const deleteUser = async (id) => {
    try {
      await apiSend('DELETE', `/users?id=${q(id)}`)
      cardOrder.value = cardOrder.value.filter(cardId => cardId !== id)
      saveCardOrder()
      await fetchUsers()
      if (emit) emit('refresh')
    } catch (err) {
      console.error('Delete user error:', err)
    }
  }

  // text уже прочитан из буфера прямо в обработчике клика (utils/clipboard.js)
  const pasteToAvatar = async (user, text) => {
    const avatar_url = String(text || '').trim()
    if (!avatar_url) return
    try {
      await apiSend('PUT', '/users', { id: user.id, name: user.name, avatar_url })
      await fetchUsers()
      if (emit) emit('refresh')
    } catch (err) {
      console.error('Paste to avatar error:', err)
    }
  }

  const pasteToName = async (user, text) => {
    const name = String(text || '').trim()
    if (!name) return
    try {
      await apiSend('PUT', '/users', { id: user.id, name, avatar_url: user.avatar_url || '' })
      await fetchUsers()
      if (emit) emit('refresh')
    } catch (err) {
      console.error('Paste to name error:', err)
    }
  }

  const onCardDragStart = (event, index) => {
    draggedCardIndex.value = index
    event.dataTransfer.effectAllowed = 'move'
    event.dataTransfer.setData('type/user-card', String(index))
  }

  const onCardDragEnd = () => {
    draggedCardIndex.value = null
    activeHoverUser.value = null
  }

  const onDragOverCard = (event, userId) => {
    event.preventDefault()
    event.dataTransfer.dropEffect = 'move'
    if (activeHoverUser.value !== userId) {
      activeHoverUser.value = userId
    }
  }

  const onDragLeaveCard = (event, userId) => {
    if (event.currentTarget && !event.currentTarget.contains(event.relatedTarget)) {
      if (activeHoverUser.value === userId) {
        activeHoverUser.value = null
      }
      onUserCardMouseLeave(userId)
    }
  }

  return {
    users,
    structures,
    cardOrder,
    draggedCardIndex,
    activeHoverUser,
    editingUserId,
    editForm,
    activeNamePasteUsers,
    saveCardOrder,
    hasChanges,
    canSave,
    fetchUsers,
    startCreateUser,
    cancelDrafts,
    startEditUser,
    cancelEdit,
    saveUser,
    deleteUser,
    pasteToAvatar,
    pasteToName,
    onNameMouseEnter,
    onNameMouseLeave,
    onUserCardMouseLeave,
    onCardDragStart,
    onCardDragEnd,
    onDragOverCard,
    onDragLeaveCard
  }
}