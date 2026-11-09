<script setup>
import { ref, computed, onMounted, watch } from 'vue'
import { cleanPersonName, cleanAccountName } from '../utils/formatters.js'

const props = defineProps({
  transactions: { type: Array, default: () => [] },
  accounts: { type: Array, default: () => [] }
})

const emit = defineEmits(['refresh'])

const API_BASE = 'http://localhost:8080/api'
const STORAGE_CUSTOM_IBANS = 'persons_custom_orphan_ibans'
const STORAGE_DELETED_IBANS = 'persons_deleted_orphan_ibans'

const users = ref([])
const searchPersonQuery = ref('')

const draggedItem = ref(null)
const activeHoverUser = ref(null)
const activeSelectIndex = ref(null)
const isGhostHovered = ref(false)

const dragGhostRef = ref(null)

// Оптимистичное удаление из таблицы при Drop
const optimisticLinkedIds = ref(new Set())

// Раскрытие IBAN и клипборд
const expandedIbans = ref(new Set())
const copiedIban = ref('')

// Состояние инлайн-редактирования / создания пользователя
const editingUserId = ref(null)
const editForm = ref({
  name: '',
  avatar_url: ''
})
const initialEditForm = ref({
  name: '',
  avatar_url: ''
})

// Состояние инлайн-добавления счетов в таблице «БЕСХОЗ»
const editingIbanItemId = ref(null)
const orphanIbanInputs = ref({})
const customOrphanIbans = ref({})
const deletedOrphanIbans = ref({})

// Сохранение и загрузка состояния БЕСХОЗ из localStorage
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

// Логика видимости и таймеров ховера иконки вставки имени
const activeNamePasteUsers = ref(new Set())
const namePasteTimers = ref({})

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

const fetchUsers = async () => {
  try {
    const res = await fetch(`${API_BASE}/users`)
    if (res.ok) {
      users.value = (await res.json()) || []
    }
  } catch (err) {
    console.error('Fetch users error:', err)
  }
}

// ВАЛИДАЦИЯ И РАСПОЗНАВАНИЕ: КАРТА (13-19 цифр) / IBAN (UA... 15-34 символов)
const validateAndCleanAccount = (input) => {
  if (!input) return null
  const clean = input.replace(/[^a-zA-Z0-9]/g, '').toUpperCase()
  
  if (/^\d{13,19}$/.test(clean)) {
    return clean
  }
  
  if (/^[A-Z]{2}\d{2}[A-Z0-9]{11,30}$/.test(clean)) {
    return clean
  }

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

// Управление добавлением и удалением счетов в разделе «БЕСХОЗ»
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

// ХОТ-ВСТАВКА С АВТО-СОХРАНЕНИЕМ В 1 КЛИК
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

// Горячее создание пользователя
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
    emit('refresh')
  } catch (err) {
    console.error('Save user error:', err)
  }
}

const deleteUser = async (id) => {
  try {
    const res = await fetch(`${API_BASE}/users?id=${encodeURIComponent(id)}`, { method: 'DELETE' })
    if (res.ok) {
      await fetchUsers()
      emit('refresh')
    }
  } catch (err) {
    console.error('Delete user error:', err)
  }
}

// Горячая вставка аватара
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
    emit('refresh')
  } catch (err) {
    console.error('Paste to avatar error:', err)
  }
}

// Горячая вставка имени
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
    emit('refresh')
  } catch (err) {
    console.error('Paste to name error:', err)
  }
}

const cacheSignaturesSet = computed(() => {
  const set = new Set()

  props.accounts.forEach(acc => {
    const name = cleanAccountName(acc.name)
    if (name) set.add(name.toLowerCase())
    if (acc.iban) set.add(acc.iban.toLowerCase())
    if (acc.masked_pan) set.add(acc.masked_pan.toLowerCase())
  })

  props.transactions.forEach(t => {
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
  }

  props.accounts.forEach(acc => {
    const accName = cleanAccountName(acc.name)
    addCandidate(accName, acc.iban || acc.masked_pan, 0)
  })

  props.transactions.forEach(t => {
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

    // Подчищаем временный локальный кэш для привязанного элемента
    delete customOrphanIbans.value[item.id]
    delete deletedOrphanIbans.value[item.id]
    saveOrphanStorage()

    activeSelectIndex.value = null
    await fetchUsers()
    emit('refresh')
  } catch (err) {
    console.error('Link signature error:', err)
  }
}

const unlinkAlias = async (user, alias) => {
  try {
    const aliasLower = (cleanPersonName(alias) || alias).toLowerCase()
    const relatedIbans = new Set()

    props.accounts.forEach(acc => {
      const accName = cleanAccountName(acc.name)
      if (accName && accName.toLowerCase() === aliasLower) {
        const iban = (acc.iban || acc.masked_pan || '').trim()
        if (iban) relatedIbans.add(iban.toLowerCase())
      }
    })

    props.transactions.forEach(t => {
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
    emit('refresh')
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

const onDragStart = (event, item) => {
  draggedItem.value = item
  event.dataTransfer.effectAllowed = 'move'
  event.dataTransfer.setData('text/plain', item.id)

  if (dragGhostRef.value) {
    event.dataTransfer.setDragImage(dragGhostRef.value, 10, 10)
  }
}

const onDragEnd = () => {
  draggedItem.value = null
  activeHoverUser.value = null
  isGhostHovered.value = false
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

const onDropOnUser = async (user) => {
  activeHoverUser.value = null
  if (!draggedItem.value) return
  const itemToLink = draggedItem.value
  
  if (user.isDraft || !user.name) {
    startCreateUser(itemToLink)
    return
  }

  optimisticLinkedIds.value.add(itemToLink.id)
  draggedItem.value = null

  await linkSignatureToUser(user, itemToLink)
  optimisticLinkedIds.value.delete(itemToLink.id)
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

const onDropOnGhost = (event) => {
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
</script>

<template>
  <main class="space-y-8">
    
    <!-- СКРЫТЫЙ СТИЛИЗОВАННЫЙ DRAG GHOST -->
    <div
      ref="dragGhostRef"
      class="fixed -left-[9999px] top-0 pointer-events-none bg-slate-900 border border-emerald-500 px-3 py-1.5 rounded text-xs font-mono text-white flex items-center gap-2 shadow-2xl z-50 shrink-0 select-none"
    >
      <span class="text-emerald-400 font-bold">⠿</span>
      <span class="font-bold text-slate-100">{{ draggedItem?.name || '' }}</span>
    </div>

    <!-- СЕТКА ЛИЦ -->
    <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
      
      <!-- КАРТОЧКИ ПОЛЬЗОВАТЕЛЕЙ -->
      <div
        v-for="user in users"
        :key="user.id"
        @dragover="onDragOverCard($event, user.id)"
        @dragenter.prevent
        @dragleave="onDragLeaveCard($event, user.id)"
        @mouseleave="onCardMouseLeave(user.id)"
        @drop.prevent="onDropOnUser(user)"
        class="bg-slate-900/70 border p-4 rounded-lg transition-all duration-300 flex flex-col justify-start min-h-[140px] relative group overflow-hidden"
        :class="[
          activeHoverUser === user.id 
            ? 'border-emerald-400 bg-slate-900 scale-[1.02] shadow-lg shadow-emerald-500/20' 
            : draggedItem 
              ? 'border-emerald-500/60 neon-border-pulse bg-emerald-950/10' 
              : 'border-slate-800'
        ]"
      >
        <!-- 1. ПОМУТНЕВШИЙ МАТОВЫЙ ОВЕРЛЕЙ ПРИ ДРАГЕ -->
        <Transition name="fade">
          <div
            v-if="draggedItem && activeHoverUser === user.id"
            class="absolute inset-0 bg-slate-950/90 backdrop-blur-sm z-10 flex flex-col items-center justify-start pt-3 pointer-events-none"
          >
            <div class="text-[10px] font-bold text-emerald-400 uppercase tracking-wider font-mono">
              ПРИВЯЗАТЬ К ЛИЦУ
            </div>
          </div>
        </Transition>

        <!-- 2. ИНЛАЙН РЕДАКТИРОВАНИЕ / СОЗДАНИЕ -->
        <template v-if="editingUserId === user.id || !user.name || user.isDraft">
          <div class="space-y-2 relative z-20">
            <input
              v-model="editForm.name"
              type="text"
              placeholder="Имя..."
              class="w-full bg-slate-950 border border-slate-800 text-xs px-2.5 py-1.5 rounded text-white outline-none focus:border-emerald-500 font-mono"
            />
            <input
              v-model="editForm.avatar_url"
              type="text"
              placeholder="URL Аватара..."
              class="w-full bg-slate-950 border border-slate-800 text-xs px-2.5 py-1.5 rounded text-white outline-none focus:border-emerald-500 font-mono"
            />

            <!-- КНОПКИ ДЕЙСТВИЙ -->
            <div class="flex items-center justify-between pt-1">
              <button
                @click="cancelEdit(user)"
                class="bg-rose-600 hover:bg-rose-500 text-white text-[10px] uppercase font-bold px-3 py-1 rounded transition-all cursor-pointer shadow-sm"
              >
                Отмена
              </button>

              <button
                @click="saveUser(user)"
                :disabled="!canSave(user)"
                class="text-[10px] uppercase font-bold px-3 py-1 rounded transition-all shadow-sm"
                :class="[
                  !canSave(user) 
                    ? 'bg-slate-800 text-slate-500 border border-slate-700/60 cursor-not-allowed' 
                    : 'bg-emerald-600 hover:bg-emerald-500 text-white cursor-pointer'
                ]"
              >
                Сохранить
              </button>
            </div>
          </div>
        </template>

        <!-- 3. ОБЫЧНЫЙ РЕЖИМ ПРОСМОТРА ПРОФИЛЯ -->
        <template v-else>
          <div
            class="relative z-20 flex items-center justify-between gap-3 transition-transform duration-300 ease-out"
            :class="draggedItem && activeHoverUser === user.id ? 'translate-y-4 translate-x-27' : 'translate-y-0 translate-x-0'"
          >
            <div class="flex items-center gap-2.5 min-w-0">
              <!-- Аватар -->
              <div
                @mousedown.prevent.stop
                @contextmenu.prevent.stop
                @click.prevent.stop="pasteToAvatar(user, $event)"
                class="relative w-9 h-9 rounded-full bg-slate-800 border border-slate-700 overflow-hidden shrink-0 flex items-center justify-center font-bold text-slate-400 text-xs select-none group/avatar cursor-pointer"
                title="Вставить URL аватара из буфера обмена"
              >
                <img v-if="user.avatar_url" :src="user.avatar_url" class="w-full h-full object-cover" />
                <span v-else>{{ user.name.charAt(0).toUpperCase() }}</span>

                <div class="absolute inset-0 bg-slate-950/75 backdrop-blur-[1px] opacity-0 group-hover/avatar:opacity-100 flex items-center justify-center transition-opacity text-slate-300 hover:text-emerald-400">
                  <svg class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                    <path d="M16 4h2a2 2 0 012 2v14a2 2 0 01-2 2H6a2 2 0 01-2-2V6a2 2 0 012-2h2" />
                    <rect x="8" y="2" width="8" height="4" rx="1" ry="1" />
                  </svg>
                </div>
              </div>

              <!-- Имя с кнопкой вставки из буфера -->
              <div
                class="flex items-center gap-1.5 min-w-0"
                @mouseenter="onNameMouseEnter(user.id)"
                @mouseleave="onNameMouseLeave(user.id)"
              >
                <span class="text-sm font-bold text-white truncate cursor-default">{{ user.name }}</span>
                
                <Transition name="slide-left">
                  <button
                    v-if="activeNamePasteUsers.has(user.id)"
                    @mousedown.prevent.stop
                    @contextmenu.prevent.stop
                    @click.prevent.stop="pasteToName(user, $event)"
                    class="text-slate-400 hover:text-emerald-400 text-[11px] transition-colors cursor-pointer p-0.5 shrink-0"
                    title="Вставить имя из буфера обмена"
                  >
                    <svg class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                      <path d="M16 4h2a2 2 0 012 2v14a2 2 0 01-2 2H6a2 2 0 01-2-2V6a2 2 0 012-2h2" />
                      <rect x="8" y="2" width="8" height="4" rx="1" ry="1" />
                    </svg>
                  </button>
                </Transition>
              </div>
            </div>

            <!-- ТУЛБАР ВЕРХНЕГО УГЛА -->
            <div class="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
              <button
                @click="startEditUser(user)"
                class="text-slate-500 hover:text-emerald-400 transition-colors p-1 cursor-pointer"
                title="Редактировать лицо"
              >
                <svg class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                  <path d="M11 4H4a2 2 0 00-2 2v14a2 2 0 002 2h14a2 2 0 002-2v-7" />
                  <path d="M18.5 2.5a2.121 2.121 0 013 3L12 15l-4 1 1-4 9.5-9.5z" />
                </svg>
              </button>

              <button
                @click="deleteUser(user.id)"
                class="text-slate-500 hover:text-rose-400 transition-colors p-1 cursor-pointer"
                title="Удалить лицо"
              >
                <svg class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                  <path d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                </svg>
              </button>
            </div>
          </div>
        </template>

        <!-- 4. ТЕГИ АЛИАСОВ И СЧЕТОВ -->
        <div class="relative z-0 mt-4 space-y-2">
          <!-- Алиасы -->
          <div v-if="(user.aliases || []).length > 0" class="space-y-1">
            <div class="text-[8px] text-slate-500 uppercase font-bold tracking-wider">Алиасы</div>
            <div class="flex flex-wrap gap-1">
              <span
                v-for="alias in user.aliases"
                :key="alias"
                class="text-[9px] px-1.5 py-0.5 rounded border font-mono flex items-center gap-1 transition-colors"
                :class="isSignatureInCache(alias) 
                  ? 'bg-slate-800 text-emerald-400 border-slate-700' 
                  : 'bg-amber-500/10 text-amber-400 border-amber-500/30'"
                :title="isSignatureInCache(alias) ? 'Подтвержден в кэше' : 'Привязан в core.db, отсутствует в текущем кэше'"
              >
                <span>{{ alias }}</span>
                <button @click.stop="unlinkAlias(user, alias)" class="text-slate-500 hover:text-rose-400 font-bold px-0.5 cursor-pointer">✕</button>
              </span>
            </div>
          </div>

          <!-- Счета -->
          <div v-if="(user.ibans || []).length > 0" class="space-y-1">
            <div class="text-[8px] text-slate-500 uppercase font-bold tracking-wider">Счета</div>
            <div class="flex flex-wrap gap-1">
              <span
                v-for="iban in user.ibans"
                :key="iban"
                @click="handleIbanClick(iban)"
                class="text-[9px] px-1.5 py-0.5 rounded border font-mono flex items-center gap-1 cursor-pointer select-none transition-all duration-300 max-w-[260px]"
                :class="isSignatureInCache(iban) 
                  ? 'bg-slate-950 text-slate-300 border-slate-800 hover:border-slate-700' 
                  : 'bg-amber-500/10 text-amber-400 border-amber-500/30'"
                :title="expandedIbans.has(iban) ? 'Скопировано! Нажмите, чтобы скрыть' : 'Нажмите, чтобы развернуть и скопировать'"
              >
                <span class="inline-block transition-all duration-300 truncate">
                  💳 {{ expandedIbans.has(iban) ? iban : formatMaskedIBAN(iban) }}
                </span>
                
                <span v-if="copiedIban === iban" class="text-emerald-400 font-bold text-[9px] animate-fade-in shrink-0">
                  ✓
                </span>
              </span>
            </div>
          </div>

          <div v-if="(!user.aliases || user.aliases.length === 0) && (!user.ibans || user.ibans.length === 0)" class="text-[10px] text-slate-600 italic">
            Перетащите алиас...
          </div>
        </div>
      </div>

      <!-- GHOST-ПЛИТКА БЫСТРОГО СОЗДАНИЯ -->
      <div
        @dragover="onDragOverGhost"
        @dragleave="onDragLeaveGhost"
        @drop.prevent="onDropOnGhost"
        class="bg-slate-900/30 border border-dashed p-4 rounded-lg flex flex-col justify-between min-h-[140px] relative transition-all"
        :class="[
          isGhostHovered 
            ? 'border-emerald-400 bg-emerald-950/20 scale-[1.01]' 
            : 'border-slate-800 hover:border-emerald-500/50'
        ]"
      >
        <div class="text-xs text-slate-500 font-mono italic">
          Перетащите контрагента или нажмите «Создать»
        </div>

        <div class="flex justify-end">
          <button
            @click="startCreateUser()"
            class="bg-emerald-600 hover:bg-emerald-500 text-white font-bold text-xs px-3 py-1.5 rounded transition-all uppercase cursor-pointer shadow-md"
          >
            Создать
          </button>
        </div>
      </div>

    </div>

    <!-- РАЗДЕЛ «БЕСХОЗ» -->
    <div class="space-y-3 pt-4">
      <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div class="text-xs font-bold text-rose-400 uppercase tracking-wider flex items-center gap-2">
          <span>БЕСХОЗ</span>
          <span class="text-[9px] px-1.5 py-0.2 bg-rose-500/10 border border-rose-500/20 rounded text-rose-400 font-mono">
            {{ orphanSignatures.length }}
          </span>
        </div>

        <div class="w-full sm:w-64">
          <input
            v-model="searchPersonQuery"
            type="text"
            placeholder="Поиск..."
            class="w-full bg-slate-900 border border-slate-800 rounded px-3 py-1.5 text-xs text-white outline-none focus:border-emerald-500 font-mono"
          />
        </div>
      </div>

      <div class="bg-slate-900/60 border border-slate-800 rounded overflow-x-auto">
        <table class="w-full text-left border-collapse text-xs">
          <thead>
            <tr class="border-b border-slate-800 text-[10px] text-slate-500 uppercase tracking-wider bg-slate-950/50">
              <th class="p-3 w-10 text-center"></th>
              <th class="p-3">АЛИАСЫ</th>
              <th class="p-3">СЧЕТА</th>
              <th class="p-3 text-center">ТРАНЗАКЦИЙ</th>
              <th class="p-3 text-right">ДЕЙСТВИЕ</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-slate-800/60 font-mono">
            <tr
              v-for="(item, idx) in orphanSignatures"
              :key="item.id"
              draggable="true"
              @dragstart="onDragStart($event, item)"
              @dragend="onDragEnd"
              class="transition-all select-none"
              :class="draggedItem && draggedItem.id === item.id 
                ? 'bg-slate-950/80 border-2 border-dashed border-slate-800' 
                : 'hover:bg-slate-800/40 cursor-grab active:cursor-grabbing'"
            >
              <template v-if="draggedItem && draggedItem.id === item.id">
                <td colspan="5" class="p-3 text-center text-slate-600 text-xs italic font-mono select-none">
                  Здесь когда-то был {{ item.name }}...
                </td>
              </template>

              <template v-else>
                <td class="p-3 text-center text-slate-500 font-bold hover:text-emerald-400 transition-colors">
                  <span class="inline-block tracking-tighter text-[11px] font-mono select-none">⠿</span>
                </td>

                <td class="p-3 font-bold text-slate-200">
                  {{ item.name }}
                </td>

                <!-- СТОЛБЕЦ «СЧЕТА» -->
                <td class="p-3 font-mono">
                  <div v-if="editingIbanItemId === item.id" class="flex items-center gap-1.5" @click.stop @mousedown.stop>
                    <div class="relative flex items-center">
                      <input
                        v-model="orphanIbanInputs[item.id]"
                        type="text"
                        placeholder="Карта или IBAN..."
                        @contextmenu.prevent.stop
                        class="bg-slate-950 border text-xs rounded px-2.5 py-1 text-white outline-none font-mono pr-7 w-48 transition-colors"
                        :class="[
                          orphanIbanInputs[item.id] && !validateAndCleanAccount(orphanIbanInputs[item.id])
                            ? 'border-rose-500/80 focus:border-rose-400'
                            : 'border-slate-700 focus:border-emerald-500'
                        ]"
                      />
                      <button
                        type="button"
                        @mousedown.prevent.stop
                        @contextmenu.prevent.stop
                        @click.prevent.stop="pasteToOrphanIbanAndSave(item, $event)"
                        class="absolute right-1 text-slate-400 hover:text-emerald-400 p-1 cursor-pointer transition-colors"
                        title="Хот-вставка из буфера с авто-сохранением"
                      >
                        <svg class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                          <path d="M16 4h2a2 2 0 012 2v14a2 2 0 01-2 2H6a2 2 0 01-2-2V6a2 2 0 012-2h2" />
                          <rect x="8" y="2" width="8" height="4" rx="1" ry="1" />
                        </svg>
                      </button>
                    </div>

                    <button
                      type="button"
                      @click.stop="cancelOrphanIban(item.id)"
                      class="bg-rose-600 hover:bg-rose-500 text-white text-[10px] uppercase font-bold px-2.5 py-1 rounded transition-all cursor-pointer shadow-sm"
                    >
                      Отмена
                    </button>

                    <button
                      type="button"
                      @click.stop="saveOrphanIban(item)"
                      :disabled="!validateAndCleanAccount(orphanIbanInputs[item.id])"
                      class="text-[10px] uppercase font-bold px-2.5 py-1 rounded transition-all shadow-sm"
                      :class="[
                        !validateAndCleanAccount(orphanIbanInputs[item.id])
                          ? 'bg-slate-800 text-slate-500 border border-slate-700/60 cursor-not-allowed' 
                          : 'bg-emerald-600 hover:bg-emerald-500 text-white cursor-pointer'
                      ]"
                    >
                      Сохранить
                    </button>
                  </div>

                  <div v-else class="flex flex-wrap items-center gap-1.5">
                    <template v-if="item.ibansArray && item.ibansArray.length > 0">
                      <span
                        v-for="iban in item.ibansArray"
                        :key="iban"
                        class="text-[10px] px-2 py-0.5 rounded border border-slate-800 bg-slate-950 text-slate-300 font-mono flex items-center gap-1.5 transition-colors hover:border-slate-700 select-none"
                      >
                        <span>{{ iban }}</span>
                        <button
                          type="button"
                          @click.stop="removeOrphanIban(item.id, iban)"
                          class="text-slate-500 hover:text-rose-400 font-bold cursor-pointer transition-colors"
                          title="Стереть счет"
                        >
                          ✕
                        </button>
                      </span>
                    </template>

                    <button
                      type="button"
                      @click.stop="startEditOrphanIban(item.id)"
                      class="text-slate-500 hover:text-emerald-400 hover:bg-slate-800 border border-slate-800 hover:border-slate-700 rounded w-5 h-5 flex items-center justify-center text-xs transition-all font-bold cursor-pointer shrink-0"
                      title="Добавить счет"
                    >
                      +
                    </button>
                  </div>
                </td>

                <td class="p-3 text-center text-slate-400 font-bold">
                  {{ item.txCount }}
                </td>

                <td class="p-3 text-right" @click.stop @mousedown.stop @touchstart.stop @dragstart.prevent>
                  <div v-if="activeSelectIndex === idx" class="flex items-center justify-end gap-1">
                    <select
                      @change="linkSignatureToUser(users.find(u => u.id === $event.target.value), item)"
                      class="bg-slate-950 border border-slate-700 text-xs rounded px-2 py-1 text-white outline-none focus:border-emerald-500 font-mono"
                    >
                      <option value="" disabled selected>Выберите лицо...</option>
                      <option v-for="u in users" :key="u.id" :value="u.id">{{ u.name }}</option>
                    </select>
                    <button
                      @click="activeSelectIndex = null"
                      class="text-slate-500 hover:text-slate-300 text-[10px] uppercase px-1 cursor-pointer"
                    >
                      ✕
                    </button>
                  </div>

                  <button
                    v-else
                    @click="activeSelectIndex = idx"
                    class="bg-slate-800 hover:bg-slate-700 text-emerald-400 border border-slate-700 font-bold text-[10px] px-2.5 py-1 rounded transition-colors uppercase cursor-pointer"
                  >
                    Привязать
                  </button>
                </td>
              </template>
            </tr>

            <tr v-if="orphanSignatures.length === 0">
              <td colspan="5" class="p-8 text-center text-slate-600 text-xs uppercase">
                Бесхозных сигнатур не найдено
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

  </main>
</template>

<style scoped>
@keyframes neonGlow {
  0%, 100% {
    border-color: rgba(16, 185, 129, 0.4);
    box-shadow: 0 0 4px rgba(16, 185, 129, 0.15);
  }
  50% {
    border-color: rgba(16, 185, 129, 0.9);
    box-shadow: 0 0 12px rgba(16, 185, 129, 0.4);
  }
}

.neon-border-pulse {
  animation: neonGlow 1.5s infinite ease-in-out;
}

.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.2s ease;
}
.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}

.slide-left-enter-active {
  transition: all 0.25s cubic-bezier(0.16, 1, 0.3, 1);
}
.slide-left-leave-active {
  transition: all 0.2s ease-in;
}
.slide-left-enter-from,
.slide-left-leave-to {
  opacity: 0;
  transform: translateX(-8px);
}
</style>