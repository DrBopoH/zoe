import { ref, computed } from 'vue'

// Постраничный показ длинного списка («Показать ещё»): rows — ref/computed со всем списком.
export function usePaged(rows, page = 100) {
  const visible = ref(page)
  const shown = computed(() => rows.value.slice(0, visible.value))
  const rest = computed(() => Math.max(0, rows.value.length - visible.value))
  const more = () => { visible.value += page }
  const reset = () => { visible.value = page }
  return { shown, rest, more, reset, page }
}
