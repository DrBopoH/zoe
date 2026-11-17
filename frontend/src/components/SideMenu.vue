<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import UiIcon from './ui/UiIcon.vue'
import UiIconButton from './ui/UiIconButton.vue'
import { SECTIONS } from '../utils/sections.js'

defineProps({ activeTab: { type: String, default: '' } })
const emit = defineEmits(['refresh', 'update:activeTab'])

const isOpen = ref(false)
const openKey = ref(SECTIONS[0]?.key ?? null) // раскрытый раздел (аккордеон); «Архив» — главная навигация, открыт сразу

const toggleSection = (key) => {
  openKey.value = openKey.value === key ? null : key
}

const openPage = (key) => {
  emit('update:activeTab', key)
  isOpen.value = false
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
      <div class="flex items-center justify-between px-4 py-3">
        <span class="flex items-center gap-2 text-sm font-bold tracking-wider text-white">
          <span class="w-2 h-2 rounded-full bg-emerald-500"></span>Ζωή
        </span>
        <UiIconButton icon="close" title="Закрыть" @click="isOpen = false" />
      </div>

      <div class="flex-1 overflow-y-auto px-3 pb-3 space-y-1">
        <div v-for="s in SECTIONS" :key="s.key" class="space-y-1">
          <button type="button" class="ui-drawer-item" :class="{ 'ui-drawer-item--active': openKey === s.key }" @click="toggleSection(s.key)">
            {{ s.label }}
            <UiIcon name="chevron" size="sm" class="ui-drawer-chevron" :class="{ 'rotate-90': openKey === s.key }" />
          </button>
          <div v-if="openKey === s.key && s.pages" class="ui-drawer-pages">
            <button
              v-for="p in s.pages"
              :key="p.key"
              type="button"
              class="ui-drawer-page"
              :class="{ 'ui-drawer-page--active': activeTab === p.key }"
              @click="openPage(p.key)"
            >
              {{ p.label }}
            </button>
          </div>
          <div v-else-if="openKey === s.key" class="ui-groups">
            <component :is="s.component" @refresh="emit('refresh')" />
          </div>
        </div>
      </div>
    </aside>
  </Transition>
</template>
