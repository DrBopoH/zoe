import { ref, computed, onMounted } from 'vue'
import { cleanPersonName, cleanAccountName } from '../utils/formatters.js'

const API_BASE = 'http://localhost:8080/api'
const STORAGE_CUSTOM_IBANS = 'persons_custom_orphan_ibans'
const STORAGE_DELETED_IBANS = 'persons_deleted_orphan_ibans'
const STORAGE_CARD_ORDER = 'persons_card_order'

export function usePersons(props, emit) {
  const users = ref([])
  const cardOrder = ref([])
  const searchPersonQuery = ref('')

  const draggedItem = ref(null)        // Элемент из «БЕСХОЗ»
  const draggedCardIndex = ref(null)   // Индекс перетаскиваемой карточки Лица
  const activeHoverUser = ref(null)
  const activeSelectIndex = ref(null)
  const isGhostHovered = ref(false)

  const optimisticLinkedIds = ref(new Set())

  const expandedIbans = ref(new Set())
  const copiedIban = ref('')

  const editingUserId = ref(null)
  const editForm = ref({ name: '', avatar_url: '' })
  const initialEditForm = ref({ name: '', avatar_url: '' })

  const editingIbanItemId = ref(null)
  const orphanIbanInputs = ref({})
  const customOrphanIbans = ref({})
  const deletedOrphanIbans = ref({})

  const activeNamePasteUsers = ref(new Set())
  const namePasteTimers = ref({})

  // Загрузка порядка карточек и состояния «БЕСХОЗ» из localStorage
  const loadOrphanStorage = () => {
    try {
      const savedCustom = localStorage.getItem(STORAGE_CUSTOM_IBANS)
      if (savedCustom) customOrphanIbans.value = JSON.parse(savedCustom)

      const savedDeleted = localStorage.getItem(STORAGE_DELETED_IBANS)
      if (savedDeleted) {
        const parsed = JSON.parse(savedDeleted)
        const restored = {}
        for (const k in parsed) {
          restored[k] = new Set(parsed[k])
        }
        deletedOrphanIbans.value = restored
      }

      const savedOrder = localStorage.getItem(STORAGE_CARD_ORDER)
      if (savedOrder) cardOrder.value = JSON.parse(savedOrder)
    } catch (err) {
      console.error('Load orphan storage error:', err)
    }
  }

  const saveOrphanStorage = () => {
    try {
      localStorage.setItem(STORAGE_CUSTOM_IBANS, JSON.stringify(customOrphanIbans.value))
      const toSaveDeleted = {}
      for (const k in deletedOrphanIbans.value) {
        toSaveDeleted[k] = Array.from(deletedOrphanIbans.value[k])
      }
      localStorage.setItem(STORAGE_DELETED_IBANS, JSON.stringify(toSaveDeleted))
    } catch (err) {
      console.error('Save orphan storage error:', err)
    }
  }

  const saveCardOrder = () => {
    try {
      localStorage.setItem(STORAGE_CARD_ORDER, JSON.stringify(cardOrder.value))
    } catch (err) {
      console.error('Save card order error:', err)
    }
  }

  // Применяем сохраненный порядок карточек (новые карточки добавляются в конец)
  const applyCardOrder = (rawUsers) => {
    // Хронологическая сортировка по умолчанию (Old -> New)
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

    // Неупорядоченные/новые добавляем в конец
    const remaining = Array.from(map.values())
    remaining.forEach(u => {
      ordered.push(u)
      cardOrder.value.push(u.id)
    })

    saveCardOrder()
    return ordered
  }

  // Таймеры ховера вставки имени
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

  // API Запросы
  const fetchUsers = async () => {
    try {
      const res = await fetch(`${API_BASE}/users`)
      if (res.ok) {
        const rawUsers = (await res.json()) || []
        users.value = applyCardOrder(rawUsers)
      }
    } catch (err) {
      console.error('Fetch users error:', err)
    }
  }

  const validateAndCleanAccount = (input) => {
    if (!input) return null
    const clean = input.replace(/[^a-zA-Z0-9]/g, '').toUpperCase()
    if (/^\d{13,19}$/.test(clean)) return clean
    if (/^[A-Z]{2}\d{2}[A-Z0-9]{11,30}$/.test(clean)) return clean
    return null
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

  // БЕСХОЗ Счета
  const startEditOrphanIban = (itemId) => {
    editingIbanItemId.value = itemId
    if (!orphanIbanInputs.value[itemId]) {
      orphanIbanInputs.value[itemId] = ''
    }
  }

  const cancelOrphanIban = (itemId) => {
    editingIbanItemId.value = null
    orphanIbanInputs.value[itemId] = ''
  }

  const removeOrphanIban = (itemId, ibanToRemove) => {
    if (!deletedOrphanIbans.value[itemId]) {
      deletedOrphanIbans.value[itemId] = new Set()
    }
    deletedOrphanIbans.value[itemId].add(ibanToRemove)

    if (customOrphanIbans.value[itemId]) {
      customOrphanIbans.value[itemId] = customOrphanIbans.value[itemId].filter(i => i !== ibanToRemove)
    }
    saveOrphanStorage()
  }

  const pasteToOrphanIbanAndSave = async (item, event) => {
    if (event) {
      event.preventDefault()
      event.stopPropagation()
    }
    try {
      const text = await navigator.clipboard.readText()
      const validAccount = validateAndCleanAccount(text)
      if (validAccount) {
        if (!customOrphanIbans.value[item.id]) {
          customOrphanIbans.value[item.id] = []
        }
        if (!customOrphanIbans.value[item.id].includes(validAccount)) {
          customOrphanIbans.value[item.id].push(validAccount)
        }
        editingIbanItemId.value = null
        orphanIbanInputs.value[item.id] = ''
        saveOrphanStorage()
      }
    } catch (err) {
      console.error('Paste to orphan IBAN error:', err)
    }
  }

  const saveOrphanIban = (item) => {
    const rawVal = orphanIbanInputs.value[item.id]
    const validAccount = validateAndCleanAccount(rawVal)
    if (!validAccount) return

    if (!customOrphanIbans.value[item.id]) {
      customOrphanIbans.value[item.id] = []
    }
    if (!customOrphanIbans.value[item.id].includes(validAccount)) {
      customOrphanIbans.value[item.id].push(validAccount)
    }
    editingIbanItemId.value = null
    orphanIbanInputs.value[item.id] = ''
    saveOrphanStorage()
  }

  // CRUD Пользователей
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

    if (droppedItem) {
      optimisticLinkedIds.value.add(droppedItem.id)
    }

    users.value.push(draftUser)
    editingUserId.value = tempId
    editForm.value = { name: '', avatar_url: '' }
    initialEditForm.value = { name: '', avatar_url: '' }
  }

  const cancelDrafts = () => {
    users.value = users.value.filter(u => {
      if (u.isDraft) {
        if (u.droppedItem) {
          optimisticLinkedIds.value.delete(u.droppedItem.id)
        }
        return false
      }
      return true
    })
  }

  const startEditUser = (user) => {
    cancelDrafts()
    editingUserId.value = user.id
    editForm.value = {
      name: user.name || '',
      avatar_url: user.avatar_url || ''
    }
    initialEditForm.value = { ...editForm.value }
  }

  const cancelEdit = (user) => {
    if (user.isDraft) {
      if (user.droppedItem) {
        optimisticLinkedIds.value.delete(user.droppedItem.id)
      }
      users.value = users.value.filter(u => u.id !== user.id)
    }
    editingUserId.value = null
    editForm.value = { name: '', avatar_url: '' }
    initialEditForm.value = { name: '', avatar_url: '' }
  }

  const saveUser = async (user) => {
    if (!canSave(user)) return

    try {
      if (user.isDraft) {
        const res = await fetch(`${API_BASE}/users`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            name: editForm.value.name.trim(),
            avatar_url: editForm.value.avatar_url.trim()
          })
        })

        if (res.ok) {
          await fetchUsers()
          if (user.droppedItem) {
            const createdUser = users.value.find(u => u.name === editForm.value.name.trim())
            if (createdUser) {
              await linkSignatureToUser(createdUser, user.droppedItem)
            }
            optimisticLinkedIds.value.delete(user.droppedItem.id)
          }
        }
      } else {
        const res = await fetch(`${API_BASE}/users`, {
          method: 'PUT',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            id: user.id,
            name: editForm.value.name.trim(),
            avatar_url: editForm.value.avatar_url.trim()
          })
        })
        if (!res.ok) {
          await fetch(`${API_BASE}/users`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
              id: user.id,
              name: editForm.value.name.trim(),
              avatar_url: editForm.value.avatar_url.trim()
            })
          })
        }
        await fetchUsers()
      }

      editingUserId.value = null
      editForm.value = { name: '', avatar_url: '' }
      initialEditForm.value = { name: '', avatar_url: '' }
      if (emit) emit('refresh')
    } catch (err) {
      console.error('Save user error:', err)
    }
  }

  const deleteUser = async (id) => {
    try {
      const res = await fetch(`${API_BASE}/users?id=${encodeURIComponent(id)}`, { method: 'DELETE' })
      if (res.ok) {
        cardOrder.value = cardOrder.value.filter(cardId => cardId !== id)
        saveCardOrder()
        await fetchUsers()
        if (emit) emit('refresh')
      }
    } catch (err) {
      console.error('Delete user error:', err)
    }
  }

  // Хот-вставки
  const pasteToAvatar = async (user, event) => {
    if (event) {
      event.preventDefault()
      event.stopPropagation()
    }
    try {
      const text = await navigator.clipboard.readText()
      const cleanText = text.replace(/[\x00-\x1F\x7F-\x9F]/g, '').trim()
      if (!cleanText) return
      const res = await fetch(`${API_BASE}/users`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          id: user.id,
          name: user.name,
          avatar_url: cleanText
        })
      })
      if (!res.ok) {
        await fetch(`${API_BASE}/users`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            id: user.id,
            name: user.name,
            avatar_url: cleanText
          })
        })
      }
      await fetchUsers()
      if (emit) emit('refresh')
    } catch (err) {
      console.error('Paste to avatar error:', err)
    }
  }

  const pasteToName = async (user, event) => {
    if (event) {
      event.preventDefault()
      event.stopPropagation()
    }
    try {
      const text = await navigator.clipboard.readText()
      const cleanText = text.replace(/[\x00-\x1F\x7F-\x9F]/g, '').trim()
      if (!cleanText) return
      const res = await fetch(`${API_BASE}/users`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          id: user.id,
          name: cleanText,
          avatar_url: user.avatar_url || ''
        })
      })
      if (!res.ok) {
        await fetch(`${API_BASE}/users`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            id: user.id,
            name: cleanText,
            avatar_url: user.avatar_url || ''
          })
        })
      }
      await fetchUsers()
      if (emit) emit('refresh')
    } catch (err) {
      console.error('Paste to name error:', err)
    }
  }

  const cacheSignaturesSet = computed(() => {
    const set = new Set();

    (props.accounts || []).forEach(acc => {
      const name = cleanAccountName(acc.name)
      if (name) set.add(name.toLowerCase())
      if (acc.iban) set.add(acc.iban.toLowerCase())
      if (acc.masked_pan) set.add(acc.masked_pan.toLowerCase())
    });

    (props.transactions || []).forEach(t => {
      const rawName = t.counter_name || t.description
      const name = cleanPersonName(rawName)
      if (name) set.add(name.toLowerCase())
      if (t.counter_iban) set.add(t.counter_iban.toLowerCase())
    })

    return set
  })

  const isSignatureInCache = (sig) => {
    if (!sig) return false
    return cacheSignaturesSet.value.has(sig.toLowerCase())
  }

  const boundAliases = computed(() => {
    const set = new Set()
    users.value.forEach(u => {
      (u.aliases || []).forEach(a => set.add(a.toLowerCase()))
    })
    return set
  })

  const boundIBANs = computed(() => {
    const set = new Set()
    users.value.forEach(u => {
      (u.ibans || []).forEach(i => set.add(i.toLowerCase()))
    })
    return set
  })

  const orphanSignatures = computed(() => {
    const map = new Map()

    const addCandidate = (name, iban, fallbackTxCount = 1) => {
      const cleanName = cleanPersonName(name)
      const cleanIbanStr = (iban || '').trim()

      if (!cleanName && !cleanIbanStr) return

      const nameKey = cleanName ? cleanName.toLowerCase() : ''
      const ibanKey = cleanIbanStr ? cleanIbanStr.toLowerCase() : ''

      if (nameKey && boundAliases.value.has(nameKey)) return
      if (ibanKey && boundIBANs.value.has(ibanKey)) return

      const groupKey = nameKey || ibanKey

      if (!map.has(groupKey)) {
        map.set(groupKey, {
          id: groupKey,
          name: cleanName || 'Неизвестный контрагент',
          rawAlias: cleanName,
          ibansSet: new Set(),
          txCount: 0
        })
      }

      const item = map.get(groupKey)
      if (cleanIbanStr) item.ibansSet.add(cleanIbanStr)
      item.txCount += fallbackTxCount
    };

    (props.accounts || []).forEach(acc => {
      const accName = cleanAccountName(acc.name)
      addCandidate(accName, acc.iban || acc.masked_pan, 0)
    });

    (props.transactions || []).forEach(t => {
      if (t.resolved_user_id) return
      const rawName = t.counter_name || t.description
      addCandidate(rawName, t.counter_iban, 1)
    })

    let list = Array.from(map.values())
      .filter(item => !optimisticLinkedIds.value.has(item.id))
      .map(item => {
        const custom = customOrphanIbans.value[item.id] || []
        const deleted = deletedOrphanIbans.value[item.id] || new Set()
        
        const allIbans = Array.from(new Set([...Array.from(item.ibansSet), ...custom]))
          .filter(i => !deleted.has(i))

        return {
          ...item,
          ibansArray: allIbans,
          ibanDisplay: allIbans.length > 0 ? allIbans.join(' · ') : ''
        }
      })

    if (searchPersonQuery.value.trim()) {
      const q = searchPersonQuery.value.toLowerCase().trim()
      list = list.filter(i => 
        i.name.toLowerCase().includes(q) || 
        i.ibanDisplay.toLowerCase().includes(q)
      )
    }

    return list.sort((a, b) => b.txCount - a.txCount)
  })

  const linkSignatureToUser = async (user, item) => {
    if (!user || !item) return
    try {
      if (item.rawAlias) {
        await fetch(`${API_BASE}/users/alias`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ user_id: user.id, alias: item.rawAlias })
        })
      }

      if (item.ibansArray && item.ibansArray.length > 0) {
        for (const iban of item.ibansArray) {
          await fetch(`${API_BASE}/users/link-account`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ user_id: user.id, account_identifier: iban, note: item.name })
          })
        }
      }

      delete customOrphanIbans.value[item.id]
      delete deletedOrphanIbans.value[item.id]
      saveOrphanStorage()

      activeSelectIndex.value = null
      await fetchUsers()
      if (emit) emit('refresh')
    } catch (err) {
      console.error('Link signature error:', err)
    }
  }

  const unlinkAlias = async (user, alias) => {
    try {
      const aliasLower = (cleanPersonName(alias) || alias).toLowerCase()
      const relatedIbans = new Set();

      (props.accounts || []).forEach(acc => {
        const accName = cleanAccountName(acc.name)
        if (accName && accName.toLowerCase() === aliasLower) {
          const iban = (acc.iban || acc.masked_pan || '').trim()
          if (iban) relatedIbans.add(iban.toLowerCase())
        }
      });

      (props.transactions || []).forEach(t => {
        const rawName = t.counter_name || t.description
        const name = cleanPersonName(rawName)
        if (name && name.toLowerCase() === aliasLower) {
          const iban = (t.counter_iban || '').trim()
          if (iban) relatedIbans.add(iban.toLowerCase())
        }
      })

      await fetch(`${API_BASE}/users/alias?alias=${encodeURIComponent(alias)}`, { method: 'DELETE' })

      const userAliasesCount = (user.aliases || []).length
      if (user.ibans && user.ibans.length > 0) {
        for (const iban of user.ibans) {
          const ibanLower = iban.toLowerCase()
          if (userAliasesCount <= 1 || relatedIbans.has(ibanLower)) {
            await fetch(`${API_BASE}/users/link-account?identifier=${encodeURIComponent(iban)}`, { method: 'DELETE' })
          }
        }
      }

      await fetchUsers()
      if (emit) emit('refresh')
    } catch (err) {
      console.error('Unlink alias error:', err)
    }
  }

  const formatMaskedIBAN = (iban) => {
    if (!iban || iban.length < 8) return iban
    return `${iban.slice(0, 4)}****${iban.slice(-4)}`
  }

  const handleIbanClick = (iban) => {
    if (expandedIbans.value.has(iban)) {
      expandedIbans.value.delete(iban)
    } else {
      expandedIbans.value.add(iban)
      navigator.clipboard.writeText(iban)
      copiedIban.value = iban
      setTimeout(() => {
        copiedIban.value = ''
      }, 1200)
    }
  }

  // Drag & Drop карточек внутри сетки
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

  const onCardMouseLeave = (userId) => {
    onUserCardMouseLeave(userId)
  }

  const onDropOnUser = async (targetIndex, user) => {
    activeHoverUser.value = null

    // 1. Привязка записи из «БЕСХОЗ»
    if (draggedItem.value) {
      const itemToLink = draggedItem.value
      if (user.isDraft || !user.name) {
        startCreateUser(itemToLink)
        return
      }

      optimisticLinkedIds.value.add(itemToLink.id)
      draggedItem.value = null

      await linkSignatureToUser(user, itemToLink)
      optimisticLinkedIds.value.delete(itemToLink.id)
      return
    }

    // 2. Перестановка карточек Лиц местами
    if (draggedCardIndex.value !== null && draggedCardIndex.value !== targetIndex) {
      const fromIndex = draggedCardIndex.value
      draggedCardIndex.value = null

      const movedUser = users.value.splice(fromIndex, 1)[0]
      users.value.splice(targetIndex, 0, movedUser)

      cardOrder.value = users.value.map(u => u.id)
      saveCardOrder()
    }
  }

  const onDragOverGhost = (event) => {
    event.preventDefault()
    event.dataTransfer.dropEffect = 'move'
    isGhostHovered.value = true
  }

  const onDragLeaveGhost = (event) => {
    if (event.currentTarget && !event.currentTarget.contains(event.relatedTarget)) {
      isGhostHovered.value = false
    }
  }

  const onDropOnGhost = () => {
    isGhostHovered.value = false
    if (!draggedItem.value) return
    const itemToLink = draggedItem.value
    draggedItem.value = null
    startCreateUser(itemToLink)
  }

  onMounted(() => {
    loadOrphanStorage()
    fetchUsers()
  })

  return {
    users,
    cardOrder,
    searchPersonQuery,
    draggedItem,
    draggedCardIndex,
    activeHoverUser,
    activeSelectIndex,
    isGhostHovered,
    optimisticLinkedIds,
    expandedIbans,
    copiedIban,
    editingUserId,
    editForm,
    editingIbanItemId,
    orphanIbanInputs,
    activeNamePasteUsers,
    hasChanges,
    canSave,
    isSignatureInCache,
    orphanSignatures,
    fetchUsers,
    validateAndCleanAccount,
    startEditOrphanIban,
    cancelOrphanIban,
    removeOrphanIban,
    saveOrphanIban,
    pasteToOrphanIbanAndSave,
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
    linkSignatureToUser,
    unlinkAlias,
    formatMaskedIBAN,
    handleIbanClick,
    onCardDragStart,
    onCardDragEnd,
    onDragOverCard,
    onDragLeaveCard,
    onCardMouseLeave,
    onDropOnUser,
    onDragOverGhost,
    onDragLeaveGhost,
    onDropOnGhost
  }
}