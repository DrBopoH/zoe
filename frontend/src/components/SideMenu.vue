<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import UiIcon from './ui/UiIcon.vue'
import UiIconButton from './ui/UiIconButton.vue'
import { SECTIONS } from '../utils/sections.js'

const emit = defineEmits(['refresh'])

const isOpen = ref(false)
const openKey = ref(null) // раскрытый раздел (аккордеон)

const toggleSection = (key) => {
  openKey.value = openKey.value === key ? null : key
}

const onKey = (e) => {
  if (e.key === 'Escape') isOpen.value = false
}
onMounted(() => window.addEventListener('keydown', onKey))
onUnmounted(() => window.removeEventListener('keydown', onKey))
</script>

<template>
  <button type="button" class="ui-burger" aria-label="Меню" @click="isOpen = true">
    <UiIcon name="menu" size="lg" />
  </button>

  <Transition name="fade">
    <div v-if="isOpen" class="ui-drawer-backdrop" data-modal @click="isOpen = false"></div>
  </Transition>

  <Transition name="drawer">
    <aside v-if="isOpen" class="ui-drawer" data-modal>
      <div class="flex items-center justify-between px-4 py-3 border-b border-slate-800">
        <span class="text-[11px] font-bold tracking-wider text-slate-300 uppercase">Разделы</span>
        <UiIconButton icon="close" title="Закрыть" @click="isOpen = false" />
      </div>

      <div class="flex-1 overflow-y-auto p-3 space-y-2">
        <div v-for="s in SECTIONS" :key="s.key" class="space-y-2">
          <button type="button" class="ui-drawer-item" :class="{ 'ui-drawer-item--active': openKey === s.key }" @click="toggleSection(s.key)">
            {{ s.label }}
            <span class="text-slate-500">{{ openKey === s.key ? '−' : '+' }}</span>
          </button>
          <div v-if="openKey === s.key" class="ui-groups">
            <component :is="s.component" @refresh="emit('refresh')" />
          </div>
        </div>
      </div>

    </aside>
  </Transition>
</template>
