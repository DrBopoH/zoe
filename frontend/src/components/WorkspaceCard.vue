<script setup>
import { computed, ref, onBeforeUnmount } from 'vue'
import UiIconButton from './ui/UiIconButton.vue'
import { coverStyle } from '../utils/workspaceCover.js'

// Карточка дела (рабочего пространства): сверху обложка — свой цвет или картинка, снизу тёмный градиент (чем выше, тем прозрачнее) и слева название.
const props = defineProps({
  workspace: { type: Object, required: true }, // { name, color, image }
  editable: { type: Boolean, default: false } // показывать карандаш и урну (в предпросмотре окна создания их нет)
})
const emit = defineEmits(['edit', 'remove'])
const cover = computed(() => coverStyle(props.workspace))

// Удаление в два клика: первый «взводит» урну на 4 секунды, второй удаляет
const armed = ref(false)
let timer = null
const onTrash = () => {
  if (armed.value) { clearTimeout(timer); armed.value = false; emit('remove', props.workspace); return }
  armed.value = true
  timer = setTimeout(() => { armed.value = false }, 4000)
}
onBeforeUnmount(() => clearTimeout(timer))
</script>

<template>
  <article class="ws-card" :title="workspace.name">
    <div class="ws-cover" :style="cover" data-testid="ws-cover"></div>
    <div class="ws-shade"></div>
    <div v-if="editable" class="ws-tools" :class="armed && 'ws-tools--armed'" @click.stop>
      <span v-if="armed" class="text-[10px] font-bold uppercase tracking-wider text-rose-300 px-1">Удалить?</span>
      <UiIconButton v-if="!armed" icon="edit" tone="accent" size="sm" title="Редактировать дело" @click="emit('edit', workspace)" />
      <UiIconButton icon="trash" tone="danger" size="sm" :title="armed ? 'Нажми ещё раз — удалить' : 'Удалить дело'" @click="onTrash" />
    </div>
    <h3 class="ws-name">{{ workspace.name }}</h3>
  </article>
</template>

<style scoped>
.ws-card {
  position: relative;
  aspect-ratio: 16 / 10;
  overflow: hidden;
  border-radius: 0.5rem;
  border: 1px solid rgb(30 41 59);
  box-shadow: 0 6px 18px rgb(0 0 0 / 0.35);
  transition: border-color 0.2s, transform 0.2s;
}
.ws-card:hover { border-color: rgb(16 185 129 / 0.45); transform: translateY(-2px); }
.ws-cover { position: absolute; inset: 0; }
/* карандаш и урна: появляются при наведении (и всегда, пока урна взведена или на карточке фокус) */
.ws-tools {
  position: absolute;
  top: 0.4rem;
  right: 0.4rem;
  display: flex;
  align-items: center;
  gap: 0.15rem;
  padding: 0.15rem;
  border-radius: 0.375rem;
  background: rgb(2 6 23 / 0.72);
  backdrop-filter: blur(4px);
  opacity: 0;
  transition: opacity 0.15s;
}
.ws-card:hover .ws-tools, .ws-card:focus-within .ws-tools, .ws-tools--armed { opacity: 1; }
/* низ темнее всего, к верху прозрачнее: обложка остаётся чистой на ~80% высоты */
.ws-shade {
  position: absolute;
  left: 0;
  right: 0;
  bottom: 0;
  height: 34%;
  background: linear-gradient(to top, rgb(2 6 23 / 0.94) 0%, rgb(2 6 23 / 0.72) 40%, rgb(2 6 23 / 0) 100%);
  pointer-events: none;
}
.ws-name {
  position: absolute;
  left: 0.7rem;
  right: 0.7rem;
  bottom: 0.5rem;
  font-size: 0.75rem;
  font-weight: 700;
  letter-spacing: 0.04em;
  color: #fff;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  text-shadow: 0 1px 2px rgb(0 0 0 / 0.5);
}
</style>
