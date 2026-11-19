<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import UiIcon from './ui/UiIcon.vue'
import UiIconButton from './ui/UiIconButton.vue'
import UiButton from './ui/UiButton.vue'
import UiGroup from './ui/UiGroup.vue'
import { SECTIONS } from '../utils/sections.js'

defineProps({ activeTab: { type: String, default: '' } })
const emit = defineEmits(['refresh', 'update:activeTab'])

const isOpen = ref(false)
const openKey = ref(null) // раскрытый раздел (аккордеон)

// Раздел-страница (tab) открывает вкладку сразу, остальные раскрываются аккордеоном
const toggleSection = (s) => {
  if (s.tab) { openPage(s.tab); return }
  openKey.value = openKey.value === s.key ? null : s.key
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

  <!-- Шапка с размытием делает себя «родителем» для fixed-потомков — панель выносится в body -->
  <Teleport to="body">
    <Transition name="fade">
      <div v-if="isOpen" class="ui-drawer-backdrop" data-modal @click="isOpen = false"></div>
    </Transition>

    <Transition name="drawer">
      <aside v-if="isOpen" class="ui-drawer" data-modal>
        <div class="flex items-center justify-between px-4 py-3 border-b border-slate-800">
          <span class="flex items-center gap-2 text-sm font-bold tracking-wider text-white">
            <span class="w-2.5 h-2.5 rounded-full bg-emerald-500"></span>Ζωή
          </span>
          <UiIconButton icon="close" title="Закрыть" @click="isOpen = false" />
        </div>

        <div class="flex-1 overflow-y-auto p-3 space-y-2">
          <div v-for="s in SECTIONS" :key="s.key" class="space-y-2">
            <button type="button" class="ui-drawer-item" :class="{ 'ui-drawer-item--active': s.tab ? activeTab === s.tab : openKey === s.key }" @click="toggleSection(s)">
              {{ s.label }}
              <UiIcon v-if="!s.tab" name="chevron" size="sm" class="ui-drawer-chevron" :class="{ 'rotate-90': openKey === s.key }" />
            </button>
            <div v-if="!s.tab && openKey === s.key" class="ui-groups">
              <UiGroup v-if="s.pages" title="Разделы">
                <div class="space-y-2">
                  <UiButton
                    v-for="p in s.pages"
                    :key="p.key"
                    :variant="activeTab === p.key ? 'subtle' : 'secondary'"
                    class="w-full"
                    @click="openPage(p.key)"
                  >{{ p.label }}</UiButton>
                </div>
              </UiGroup>
              <component :is="s.component" v-else @refresh="emit('refresh')" />
            </div>
          </div>
        </div>
      </aside>
    </Transition>
  </Teleport>
</template>
