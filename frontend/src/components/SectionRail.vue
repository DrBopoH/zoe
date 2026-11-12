<script setup>
import { ref, computed, watch } from 'vue'
import UiIcon from './ui/UiIcon.vue'
import { storageGet, storageSet } from '../utils/storage.js'

// Список разделов вкладки. Стоит ЗА основной сеткой (в левом поле), чтобы не сдвигать и не сужать экран: тот же размер, что у Сметы.
// Широкое поле (окно от 1380px) — подписи; среднее (от 1240px) — узкая колонка иконок; на узком окне — строка сверху.
// Родитель обязан быть `relative`. sections: [{ key, label, icon, count? }] — раздел — либо лист, либо группа { key, label, icon, children: [лист] }.
// Группа раскрывается и сворачивается только в широком режиме (там у неё есть заголовок); в узких режимах листья групп идут подряд.
// Открытость групп помнится под ключом groupsKey; выбранный раздел всегда виден (его группа раскрывается сама).
const props = defineProps({
  sections: { type: Array, required: true },
  modelValue: { type: String, required: true },
  label: { type: String, default: 'Разделы' },
  groupsKey: { type: String, default: '' }
})
const emit = defineEmits(['update:modelValue'])

const load = () => {
  if (!props.groupsKey) return {}
  try { return JSON.parse(storageGet(props.groupsKey, '{}')) || {} } catch (e) { return {} }
}
const closed = ref(load()) // key группы → true, если свёрнута
const isOpen = (g) => !closed.value[g.key] || (g.children || []).some((c) => c.key === props.modelValue)
const toggle = (g) => {
  closed.value = { ...closed.value, [g.key]: isOpen(g) }
  if (props.groupsKey) storageSet(props.groupsKey, JSON.stringify(closed.value))
}
// выбранный раздел внутри свёрнутой группы: раскрываем группу насовсем, иначе после смены раздела она схлопнулась бы под курсором
watch(() => props.modelValue, (k) => {
  const g = props.sections.find((s) => s.children?.some((c) => c.key === k))
  if (g && closed.value[g.key]) { closed.value = { ...closed.value, [g.key]: false }; if (props.groupsKey) storageSet(props.groupsKey, JSON.stringify(closed.value)) }
})
const rows = computed(() => props.sections)
</script>

<template>
  <div class="min-[1240px]:absolute min-[1240px]:right-full min-[1240px]:top-0 min-[1240px]:bottom-0 min-[1240px]:mr-2 min-[1240px]:w-9 min-[1380px]:mr-3 min-[1380px]:w-24 mb-4 min-[1240px]:mb-0">
    <nav class="flex min-[1240px]:flex-col gap-1 text-xs min-[1240px]:sticky min-[1240px]:top-20" :aria-label="label">
      <template v-for="s in rows" :key="s.key">
        <template v-if="!s.children">
          <button
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
        </template>
        <template v-else>
          <button
            type="button"
            :title="s.label"
            :aria-expanded="isOpen(s)"
            @click="toggle(s)"
            class="hidden min-[1380px]:flex px-3 pt-3 pb-1 text-[10px] uppercase font-bold tracking-wider text-slate-500 hover:text-slate-300 cursor-pointer items-center justify-between gap-1 text-left"
          >
            <span>{{ s.label }}</span>
            <UiIcon name="chevron" size="sm" class="transition-transform" :class="isOpen(s) ? 'rotate-90' : ''" />
          </button>
          <div class="contents" :class="isOpen(s) ? 'min-[1380px]:flex min-[1380px]:flex-col min-[1380px]:gap-1' : 'min-[1380px]:hidden'">
            <button
              v-for="c in s.children"
              :key="c.key"
              type="button"
              :title="c.count != null ? `${c.label} · ${c.count}` : c.label"
              @click="emit('update:modelValue', c.key)"
              class="px-3 py-2 min-[1240px]:px-0 min-[1380px]:px-3 rounded transition-colors uppercase font-bold cursor-pointer flex items-center justify-between min-[1240px]:justify-center min-[1380px]:justify-between gap-2 text-left"
              :class="modelValue === c.key ? 'bg-slate-800 text-emerald-400' : 'text-slate-400 hover:text-slate-200'"
            >
              <UiIcon :name="c.icon" size="lg" class="hidden min-[1240px]:block min-[1380px]:hidden" />
              <span class="min-[1240px]:hidden min-[1380px]:inline">{{ c.label }}</span>
              <span v-if="c.count != null" class="text-[9px] px-1.5 bg-slate-900 border border-slate-700 rounded text-slate-400 min-[1240px]:hidden min-[1380px]:inline">{{ c.count }}</span>
            </button>
          </div>
        </template>
      </template>
    </nav>
  </div>
</template>
