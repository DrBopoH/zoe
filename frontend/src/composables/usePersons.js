import { ref, computed, onMounted } from 'vue'
import { usePersonsData } from './usePersonsData.js'
import { useOrphans } from './useOrphans.js'
import { useLinkEditor } from './useLinkEditor.js'
import { buildAliasGraph } from '../utils/personLayout.js'
import { pushToast } from './useToasts.js'

export function usePersons(props, emit) {
  const expandedIbans = ref(new Set())
  const copiedIban = ref('')

  const personsData = usePersonsData(emit)

  // Единый граф алиасов: кэш (счета/транзакции) ∪ статика (лица, структуры, баны)
  const graph = computed(() => buildAliasGraph({
    accounts: props.accounts,
    transactions: props.transactions,
    users: personsData.users.value,
    structures: personsData.structures.value
  }))

  const orphans = useOrphans(props, personsData.users, emit, personsData.fetchUsers, graph)
  const linkEditor = useLinkEditor(personsData.users, emit, personsData.fetchUsers)

  const handleIbanClick = (iban) => {
    if (expandedIbans.value.has(iban)) {
      expandedIbans.value.delete(iban)
    } else {
      expandedIbans.value.add(iban)
      navigator.clipboard?.writeText(iban).catch(() => {})
      copiedIban.value = iban
      pushToast({ kind: 'ok', title: 'Скопировано', ttl: 1500 })
      setTimeout(() => {
        copiedIban.value = ''
      }, 1200)
    }
  }

  const cancelEdit = (user) => {
    personsData.cancelEdit(user, (droppedItemId) => {
      orphans.optimisticLinkedIds.value.delete(droppedItemId)
    })
  }

  const cancelDrafts = () => {
    personsData.cancelDrafts((droppedItemId) => {
      orphans.optimisticLinkedIds.value.delete(droppedItemId)
    })
  }

  const startCreateUser = (droppedItem = null) => {
    if (droppedItem) {
      orphans.optimisticLinkedIds.value.add(droppedItem.id)
    }
    return personsData.startCreateUser(droppedItem)
  }

  const saveUser = (user) => {
    return personsData.saveUser(user, async (createdUser, droppedItem) => {
      await orphans.linkSignatureToUser(createdUser, droppedItem)
      orphans.optimisticLinkedIds.value.delete(droppedItem.id)
    })
  }

  const onDropOnUser = async (targetIndex, user) => {
    personsData.activeHoverUser.value = null

    if (orphans.draggedItem.value) {
      const itemToLink = orphans.draggedItem.value
      if (user.isDraft || !user.name) {
        startCreateUser(itemToLink)
        return
      }

      orphans.optimisticLinkedIds.value.add(itemToLink.id)
      orphans.draggedItem.value = null

      await orphans.linkSignatureToUser(user, itemToLink)
      orphans.optimisticLinkedIds.value.delete(itemToLink.id)
      return
    }

    if (personsData.draggedCardIndex.value !== null && personsData.draggedCardIndex.value !== targetIndex) {
      const fromIndex = personsData.draggedCardIndex.value
      personsData.draggedCardIndex.value = null

      const movedUser = personsData.users.value.splice(fromIndex, 1)[0]
      personsData.users.value.splice(targetIndex, 0, movedUser)

      personsData.cardOrder.value = personsData.users.value.map(u => u.id)
      personsData.saveCardOrder()
    }
  }

  const onDragOverGhost = (event) => {
    event.preventDefault()
    event.dataTransfer.dropEffect = 'move'
    orphans.isGhostHovered.value = true
  }

  const onDragLeaveGhost = (event) => {
    if (event.currentTarget && !event.currentTarget.contains(event.relatedTarget)) {
      orphans.isGhostHovered.value = false
    }
  }

  const onDropOnGhost = () => {
    orphans.isGhostHovered.value = false
    if (!orphans.draggedItem.value) return
    const itemToLink = orphans.draggedItem.value
    orphans.draggedItem.value = null
    startCreateUser(itemToLink)
  }

  onMounted(() => {
    personsData.fetchUsers()
  })

  return {
    // Реактивные данные и состояние
    users: personsData.users,
    graph,
    cardOrder: personsData.cardOrder,
    searchPersonQuery: orphans.searchPersonQuery,
    draggedItem: orphans.draggedItem,
    draggedCardIndex: personsData.draggedCardIndex,
    activeHoverUser: personsData.activeHoverUser,
    activeSelectIndex: orphans.activeSelectIndex,
    isGhostHovered: orphans.isGhostHovered,
    optimisticLinkedIds: orphans.optimisticLinkedIds,
    expandedIbans,
    copiedIban,
    editingUserId: personsData.editingUserId,
    editForm: personsData.editForm,
    activeNamePasteUsers: personsData.activeNamePasteUsers,
    isModalOpen: linkEditor.isModalOpen,
    modalAliasData: linkEditor.modalAliasData,

    // Вычисляемые свойства
    hasChanges: personsData.hasChanges,
    canSave: personsData.canSave,
    isSignatureInCache: orphans.isSignatureInCache,
    orphanSignatures: orphans.orphanSignatures,

    // Методы
    fetchUsers: personsData.fetchUsers,
    openLinkEditor: linkEditor.openLinkEditor,
    closeLinkEditor: linkEditor.closeLinkEditor,
    quickCreateUser: linkEditor.quickCreateUser,
    saveLinkEditorData: linkEditor.saveLinkEditorData,
    startCreateUser,
    cancelDrafts,
    startEditUser: personsData.startEditUser,
    cancelEdit,
    saveUser,
    deleteUser: personsData.deleteUser,
    pasteToAvatar: personsData.pasteToAvatar,
    pasteToName: personsData.pasteToName,
    onNameMouseEnter: personsData.onNameMouseEnter,
    onNameMouseLeave: personsData.onNameMouseLeave,
    onUserCardMouseLeave: personsData.onUserCardMouseLeave,
    linkSignatureToUser: orphans.linkSignatureToUser,
    unlinkAlias: orphans.unlinkAlias,
    handleIbanClick,
    onCardDragStart: personsData.onCardDragStart,
    onCardDragEnd: personsData.onCardDragEnd,
    onDragOverCard: personsData.onDragOverCard,
    onDragLeaveCard: personsData.onDragLeaveCard,
    onCardMouseLeave: personsData.onUserCardMouseLeave,
    onDropOnUser,
    onDragOverGhost,
    onDragLeaveGhost,
    onDropOnGhost
  }
}
