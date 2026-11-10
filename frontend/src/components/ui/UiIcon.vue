<script setup>
import { computed } from 'vue'

const props = defineProps({
  name: { type: String, required: true },
  size: { type: String, default: 'md' } // sm | md | lg
})

// Все иконки проекта в одном месте (раньше SVG копировались по компонентам)
const SHAPES = {
  edit: [
    { d: 'M11 4H4a2 2 0 00-2 2v14a2 2 0 002 2h14a2 2 0 002-2v-7' },
    { d: 'M18.5 2.5a2.121 2.121 0 013 3L12 15l-4 1 1-4 9.5-9.5z' }
  ],
  close: [{ d: 'M18 6L6 18' }, { d: 'M6 6l12 12' }],
  menu: [{ d: 'M4 6h16' }, { d: 'M4 12h16' }, { d: 'M4 18h16' }],
  check: [{ d: 'M5 12l5 5L20 7' }],
  arrowUp: [{ d: 'M12 19V5' }, { d: 'M5 12l7-7 7 7' }],
  arrowDown: [{ d: 'M12 5v14' }, { d: 'M19 12l-7 7-7-7' }],
  plus: [{ d: 'M12 5v14' }, { d: 'M5 12h14' }],
  trash: [
    { d: 'M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16' }
  ],
  paste: [
    { d: 'M16 4h2a2 2 0 012 2v14a2 2 0 01-2 2H6a2 2 0 01-2-2V6a2 2 0 012-2h2' },
    { rect: { x: 8, y: 2, width: 8, height: 4, rx: 1, ry: 1 } }
  ]
}

const SIZES = { sm: 'w-3 h-3', md: 'w-3.5 h-3.5', lg: 'w-4 h-4' }

const shapes = computed(() => SHAPES[props.name] || [])
const sizeClass = computed(() => SIZES[props.size] || SIZES.md)
</script>

<template>
  <svg
    :class="sizeClass"
    viewBox="0 0 24 24"
    fill="none"
    stroke="currentColor"
    stroke-width="2"
    stroke-linecap="round"
    stroke-linejoin="round"
    aria-hidden="true"
  >
    <template v-for="(s, i) in shapes" :key="i">
      <path v-if="s.d" :d="s.d" />
      <rect v-else-if="s.rect" v-bind="s.rect" />
    </template>
  </svg>
</template>
