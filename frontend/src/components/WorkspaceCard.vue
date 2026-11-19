<script setup>
import { computed } from 'vue'
import { coverStyle } from '../utils/workspaceCover.js'

// Карточка дела (рабочего пространства): сверху обложка — свой цвет или картинка, снизу тёмный градиент (чем выше, тем прозрачнее) и слева название.
const props = defineProps({
  workspace: { type: Object, required: true } // { name, color, image }
})
const cover = computed(() => coverStyle(props.workspace))
</script>

<template>
  <article class="ws-card" :title="workspace.name">
    <div class="ws-cover" :style="cover" data-testid="ws-cover"></div>
    <div class="ws-shade"></div>
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
  left: 0.9rem;
  right: 0.9rem;
  bottom: 0.7rem;
  font-size: 0.875rem;
  font-weight: 700;
  letter-spacing: 0.04em;
  color: #fff;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  text-shadow: 0 1px 2px rgb(0 0 0 / 0.5);
}
</style>
