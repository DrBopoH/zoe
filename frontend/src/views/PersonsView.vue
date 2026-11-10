<script setup>
import { ref, watch, computed } from 'vue'
import { usePersons } from '../composables/usePersons.js'
import { useDragAutoScroll } from '../composables/useDragAutoScroll.js'
import DragEdgeZones from '../components/DragEdgeZones.vue'
import PersonCard from '../components/PersonCard.vue'
import OrphanTable from '../components/OrphanTable.vue'
import AliasDrumModal from '../components/AliasDrumModal.vue'
import LinkEditorModal from '../components/LinkEditorModal.vue'
import UiButton from '../components/ui/UiButton.vue'

const props = defineProps({
  transactions: { type: Array, default: () => [] },
  accounts: { type: Array, default: () => [] },
  // версия данных бэкенда: растёт при любой записи в кэш/статику (импорт, синк, правки)
  dataVersion: { type: Number, default: null }
})

const emit = defineEmits(['refresh'])

const dragGhostRef = ref(null)
const drumAlias = ref('') // алиас, открытый в модалке-барабане

const {
  users,
  graph,
  fetchUsers,
  searchPersonQuery,
  draggedItem,
  draggedCardIndex,
  activeHoverUser,
  activeSelectIndex,
  isGhostHovered,
  expandedIbans,
  copiedIban,
  editingUserId,
  editForm,
  activeNamePasteUsers,
  isModalOpen,
  modalAliasData,
  canSave,
  isSignatureInCache,
  orphanSignatures,
  openLinkEditor,
  closeLinkEditor,
  quickCreateUser,
  saveLinkEditorData,
  startCreateUser,
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
  handleIbanClick,
  onCardDragStart,
  onCardDragEnd,
  onDragOverCard,
  onDragLeaveCard,
  onDropOnUser,
  onDragOverGhost,
  onDragLeaveGhost,
  onDropOnGhost
} = usePersons(props, emit)

// автопрокрутка у краёв экрана, пока тащим бесхоз или карточку лица
const isDragging = computed(() => !!draggedItem.value || draggedCardIndex.value !== null)
const { edge } = useDragAutoScroll(isDragging)

// Бэкенд сам обновляет статику при импорте/синке (syncAliases) — подтягиваем лица и структуры
watch(() => props.dataVersion, (v, old) => {
  if (old !== null && v !== old) fetchUsers()
})

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
</script>

<template>
  <main class="space-y-8 relative">
    <!-- ЕДИНОЕ МОДАЛЬНОЕ ОКНО РЕДАКТИРОВАНИЯ СВЯЗЕЙ -->
	<LinkEditorModal
	  :is-open="isModalOpen"
	  :alias-data="modalAliasData"
	  :users="users"
	  :accounts="accounts"
	  :is-signature-in-cache="isSignatureInCache"
	  @close="closeLinkEditor"
	  @save-link="saveLinkEditorData"
	  @quick-create-user="quickCreateUser"
	/>

    <AliasDrumModal
      :is-open="!!drumAlias"
      :alias="drumAlias"
      :cached="isSignatureInCache(drumAlias)"
      :graph="graph"
      :accounts="accounts"
      :transactions="transactions"
      @close="drumAlias = ''"
      @changed="fetchUsers"
    />

    <DragEdgeZones :edge="edge" />

    <!-- СКРЫТЫЙ СТИЛИЗОВАННЫЙ DRAG GHOST БЕСХОЗА -->
    <div
      ref="dragGhostRef"
      class="fixed -left-[9999px] top-0 pointer-events-none bg-slate-900 border border-emerald-500 px-3 py-1.5 rounded text-xs font-mono text-white flex items-center gap-2 shadow-2xl z-50 shrink-0 select-none"
    >
      <span class="text-emerald-400 font-bold">⠿</span>
      <span class="font-bold text-slate-100">{{ draggedItem?.name || '' }}</span>
    </div>

    <!-- СЕТКА ЛИЦ -->
    <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
      <PersonCard
        v-for="(user, index) in users"
        :key="user.id"
        :user="user"
        :index="index"
        :dragged-item="draggedItem"
        :dragged-card-index="draggedCardIndex"
        :active-hover-user="activeHoverUser"
        :editing-user-id="editingUserId"
        :edit-form="editForm"
        :active-name-paste-users="activeNamePasteUsers"
        :expanded-ibans="expandedIbans"
        :copied-iban="copiedIban"
        :can-save="canSave"
        :is-signature-in-cache="isSignatureInCache"
        :graph="graph"
        @card-dragstart="onCardDragStart"
        @card-dragend="onCardDragEnd"
        @dragover-card="onDragOverCard"
        @dragleave-card="onDragLeaveCard"
        @mouseleave-card="onUserCardMouseLeave"
        @drop-user="onDropOnUser"
        @cancel-edit="cancelEdit"
        @save-user="saveUser"
        @paste-avatar="pasteToAvatar"
        @mouseenter-name="onNameMouseEnter"
        @mouseleave-name="onNameMouseLeave"
        @paste-name="pasteToName"
        @edit-user="startEditUser"
        @delete-user="deleteUser"
        @unlink-alias="unlinkAlias"
        @click-iban="handleIbanClick"
        @open-link-editor="openLinkEditor"
        @open-alias-drum="drumAlias = $event"
      />

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
          <UiButton variant="primary" @click="startCreateUser()">Создать</UiButton>
        </div>
      </div>
    </div>

    <!-- РАЗДЕЛ «БЕСХОЗ» -->
    <OrphanTable
      v-model:search-person-query="searchPersonQuery"
      v-model:active-select-index="activeSelectIndex"
      :orphan-signatures="orphanSignatures"
      :users="users"
      :dragged-item="draggedItem"
      :is-signature-in-cache="isSignatureInCache"
      @drag-start="onDragStart"
      @drag-end="onDragEnd"
      @link-signature="linkSignatureToUser"
      @open-link-editor="openLinkEditor"
    />
  </main>
</template>
