<script setup>
import { ref } from 'vue'
import { usePersons } from '../composables/usePersons.js'
import PersonCard from '../components/PersonCard.vue'
import OrphanTable from '../components/OrphanTable.vue'

const props = defineProps({
  transactions: { type: Array, default: () => [] },
  accounts: { type: Array, default: () => [] }
})

const emit = defineEmits(['refresh'])

const dragGhostRef = ref(null)

const persons = usePersons(props, emit)

const onDragStart = (event, item) => {
  persons.draggedItem.value = item
  event.dataTransfer.effectAllowed = 'move'
  event.dataTransfer.setData('text/plain', item.id)

  if (dragGhostRef.value) {
    event.dataTransfer.setDragImage(dragGhostRef.value, 10, 10)
  }
}

const onDragEnd = () => {
  persons.draggedItem.value = null
  persons.activeHoverUser.value = null
  persons.isGhostHovered.value = false
}
</script>

<template>
  <main class="space-y-8">
    <!-- СКРЫТЫЙ СТИЛИЗОВАННЫЙ DRAG GHOST БЕСХОЗА -->
    <div
      ref="dragGhostRef"
      class="fixed -left-[9999px] top-0 pointer-events-none bg-slate-900 border border-emerald-500 px-3 py-1.5 rounded text-xs font-mono text-white flex items-center gap-2 shadow-2xl z-50 shrink-0 select-none"
    >
      <span class="text-emerald-400 font-bold">⠿</span>
      <span class="font-bold text-slate-100">{{ persons.draggedItem.value?.name || '' }}</span>
    </div>

    <!-- СЕТКА ЛИЦ -->
    <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
      <PersonCard
        v-for="(user, index) in persons.users.value"
        :key="user.id"
        :user="user"
        :index="index"
        :dragged-item="persons.draggedItem.value"
        :dragged-card-index="persons.draggedCardIndex.value"
        :active-hover-user="persons.activeHoverUser.value"
        :editing-user-id="persons.editingUserId.value"
        :edit-form="persons.editForm.value"
        :active-name-paste-users="persons.activeNamePasteUsers.value"
        :expanded-ibans="persons.expandedIbans.value"
        :copied-iban="persons.copiedIban.value"
        :can-save="persons.canSave"
        :is-signature-in-cache="persons.isSignatureInCache"
        :format-masked-i-b-a-n="persons.formatMaskedIBAN"
        @card-dragstart="persons.onCardDragStart"
        @card-dragend="persons.onCardDragEnd"
        @dragover-card="persons.onDragOverCard"
        @dragleave-card="persons.onDragLeaveCard"
        @mouseleave-card="persons.onCardMouseLeave"
        @drop-user="persons.onDropOnUser"
        @cancel-edit="persons.cancelEdit"
        @save-user="persons.saveUser"
        @paste-avatar="persons.pasteToAvatar"
        @mouseenter-name="persons.onNameMouseEnter"
        @mouseleave-name="persons.onNameMouseLeave"
        @paste-name="persons.pasteToName"
        @edit-user="persons.startEditUser"
        @delete-user="persons.deleteUser"
        @unlink-alias="persons.unlinkAlias"
        @click-iban="persons.handleIbanClick"
      />

      <!-- GHOST-ПЛИТКА БЫСТРОГО СОЗДАНИЯ -->
      <div
        @dragover="persons.onDragOverGhost"
        @dragleave="persons.onDragLeaveGhost"
        @drop.prevent="persons.onDropOnGhost"
        class="bg-slate-900/30 border border-dashed p-4 rounded-lg flex flex-col justify-between min-h-[140px] relative transition-all"
        :class="[
          persons.isGhostHovered.value 
            ? 'border-emerald-400 bg-emerald-950/20 scale-[1.01]' 
            : 'border-slate-800 hover:border-emerald-500/50'
        ]"
      >
        <div class="text-xs text-slate-500 font-mono italic">
          Перетащите контрагента или нажмите «Создать»
        </div>

        <div class="flex justify-end">
          <button
            @click="persons.startCreateUser()"
            class="bg-emerald-600 hover:bg-emerald-500 text-white font-bold text-xs px-3 py-1.5 rounded transition-all uppercase cursor-pointer shadow-md"
          >
            Создать
          </button>
        </div>
      </div>
    </div>

    <!-- РАЗДЕЛ «БЕСХОЗ» -->
    <OrphanTable
      v-model:search-person-query="persons.searchPersonQuery.value"
      v-model:active-select-index="persons.activeSelectIndex.value"
      :orphan-signatures="persons.orphanSignatures.value"
      :users="persons.users.value"
      :dragged-item="persons.draggedItem.value"
      :editing-iban-item-id="persons.editingIbanItemId.value"
      :orphan-iban-inputs="persons.orphanIbanInputs.value"
      :validate-and-clean-account="persons.validateAndCleanAccount"
      @drag-start="onDragStart"
      @drag-end="onDragEnd"
      @paste-iban-save="persons.pasteToOrphanIbanAndSave"
      @cancel-orphan-iban="persons.cancelOrphanIban"
      @save-orphan-iban="persons.saveOrphanIban"
      @remove-orphan-iban="persons.removeOrphanIban"
      @start-edit-orphan-iban="persons.startEditOrphanIban"
      @link-signature="persons.linkSignatureToUser"
    />
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