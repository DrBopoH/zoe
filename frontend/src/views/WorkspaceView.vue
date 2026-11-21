<script setup>
import { computed, defineAsyncComponent } from 'vue'
import UiIcon from '../components/ui/UiIcon.vue'
import UiEmpty from '../components/ui/UiEmpty.vue'
import { coverStyle } from '../utils/workspaceCover.js'
import { pageOf } from '../utils/workspacePages.js'

// Страница одного дела: шапка из данных карточки (обложка, название) и под ней страница из реестра по ключу дела.
// Нет ключа или страница не зарегистрирована — заглушка, дело при этом открывается.
const props = defineProps({
  workspace: { type: Object, required: true } // { id, name, color, image, page }
})
defineEmits(['back'])

const cover = computed(() => coverStyle(props.workspace))
const entry = computed(() => pageOf(props.workspace.page))
// компонент страницы: пересоздаётся только при смене записи реестра, не при каждой правке дела
const Page = computed(() => (entry.value ? defineAsyncComponent(entry.value.load) : null))
</script>

<template>
  <div class="space-y-4">
    <header class="ws-head" data-testid="ws-head">
      <div class="ws-head-cover" :style="cover"></div>
      <div class="ws-head-shade"></div>
      <button type="button" class="ws-back" title="Ко всем делам" aria-label="Ко всем делам" @click="$emit('back')">
        <span class="inline-flex rotate-180"><UiIcon name="chevron" /></span>
        <span>Дела</span>
      </button>
      <h2 class="ws-head-name">{{ workspace.name }}</h2>
    </header>

    <component :is="Page" v-if="Page" :key="workspace.page" :workspace="workspace" />
    <UiEmpty v-else>У этого дела пока нет страницы. Выбрать её можно в правке (карандаш на карточке)</UiEmpty>
  </div>
</template>

<style scoped>
.ws-head {
  position: relative;
  height: 9rem;
  overflow: hidden;
  border-radius: 0.5rem;
  border: 1px solid rgb(30 41 59);
}
.ws-head-cover { position: absolute; inset: 0; }
.ws-head-shade {
  position: absolute;
  inset: auto 0 0 0;
  height: 60%;
  background: linear-gradient(to top, rgb(2 6 23 / 0.94) 0%, rgb(2 6 23 / 0.6) 50%, rgb(2 6 23 / 0) 100%);
  pointer-events: none;
}
.ws-head-name {
  position: absolute;
  left: 1rem;
  right: 1rem;
  bottom: 0.8rem;
  font-size: 1.25rem;
  font-weight: 700;
  letter-spacing: 0.04em;
  color: #fff;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  text-shadow: 0 1px 2px rgb(0 0 0 / 0.5);
}
.ws-back {
  position: absolute;
  top: 0.6rem;
  left: 0.6rem;
  display: inline-flex;
  align-items: center;
  gap: 0.15rem;
  padding: 0.25rem 0.6rem 0.25rem 0.35rem;
  border-radius: 0.375rem;
  background: rgb(2 6 23 / 0.72);
  backdrop-filter: blur(4px);
  color: rgb(203 213 225);
  font-size: 0.7rem;
  font-weight: 700;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  cursor: pointer;
  transition: color 0.15s;
}
.ws-back:hover { color: rgb(110 231 183); }
</style>
