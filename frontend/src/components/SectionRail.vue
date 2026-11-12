<script setup>
import UiIcon from './ui/UiIcon.vue'

// Список разделов вкладки. Стоит ЗА основной сеткой (в левом поле), чтобы не сдвигать и не сужать экран: тот же размер, что у Сметы.
// Широкое поле (окно от 1380px) — подписи; среднее (от 1240px) — узкая колонка иконок; на узком окне — строка сверху.
// Родитель обязан быть `relative`. sections: [{ key, label, icon, count? }].
defineProps({
  sections: { type: Array, required: true },
  modelValue: { type: String, required: true },
  label: { type: String, default: 'Разделы' }
})
const emit = defineEmits(['update:modelValue'])
</script>

<template>
  <div class="min-[1240px]:absolute min-[1240px]:right-full min-[1240px]:top-0 min-[1240px]:bottom-0 min-[1240px]:mr-2 min-[1240px]:w-9 min-[1380px]:mr-3 min-[1380px]:w-24 mb-4 min-[1240px]:mb-0">
    <nav class="flex min-[1240px]:flex-col gap-1 text-xs min-[1240px]:sticky min-[1240px]:top-20" :aria-label="label">
      <button
        v-for="s in sections"
        :key="s.key"
        type="button"
        :title="s.count != null ? `${s.label} · ${s.count}` : s.label"
        @click="emit('update:modelValue', s.key)"
        class="px-3 py-2 min-[1240px]:px-0 min-[1380px]:px-3 rounded transition-colors uppercase font-bold cursor-pointer flex items-center justify-between min-[1240px]:justify-center min-[1380px]:justify-between gap-2 text-left"
        :class="modelValue === s.key ? 'bg-slate-800 text-emerald-400' : 'text-slate-400 hover:text-slate-200'"
      >
        <UiIcon :name="s.icon" size="lg" class="hidden min-[1240px]:block min-[1380px]:hidden" />
        <span class="min-[1240px]:hidden min-[1380px]:inline">{{ s.label }}</span>
        <span v-if="s.count != null" class="text-[9px] px-1.5 bg-slate-900 border border-slate-700 rounded text-slate-400 min-[1240px]:hidden min-[1380px]:inline">{{ s.count }}</span>
      </button>
    </nav>
  </div>
</template>
