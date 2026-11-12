<script setup>
import { ref, computed, nextTick } from 'vue'
import { formatMaskedIBAN } from '../../utils/formatters.js'
import { parseMask, flagEmoji, bankMark, binanceMark, failedLogos } from '../../utils/ibanMask.js'
import { writeClipboardText } from '../../utils/clipboard.js'
import { useIbanMasks } from '../../composables/useIbanMasks.js'
import { pushToast } from '../../composables/useToasts.js'
import UiIcon from './UiIcon.vue'

// Номер счёта или карты. Показ: скрытый вид (для IBAN — флаг, контрольные цифры, значок банка, 4 первые … 4 последние
// или своё имя счёта). interactive: клик разворачивает полный номер и копирует его (как в Лицах), карандаш правее — псевдоним.
const props = defineProps({
  id: { type: String, default: '' },
  interactive: { type: Boolean, default: false },
  compact: { type: Boolean, default: false } // узкая плитка: только последние четыре цифры
})

const { maskOf, saveMask } = useIbanMasks()
const norm = computed(() => String(props.id || '').replace(/[^a-z0-9]/gi, '').toUpperCase())
// IBAN или профиль Binance (BN + цифры): им генерируется скрытый вид и доступен свой псевдоним
const isIban = computed(() => /^BN\d{6,12}$/.test(norm.value) || (/^[A-Z]{2}\d{2}[A-Z0-9]{11,30}$/.test(norm.value) && !norm.value.startsWith('BN')))
const mask = computed(() => (isIban.value ? maskOf(props.id) : null))
const parsed = computed(() => (mask.value ? parseMask(mask.value.masked) : null))
const bank = computed(() => (parsed.value ? (parsed.value.binance ? binanceMark() : bankMark(parsed.value.bank)) : null))
// хвост номера: полный вид «0000****9976» (у Binance «****0041»), в узкой плитке только четыре цифры
const tailText = computed(() => {
  const p = parsed.value
  if (!p) return ''
  return (props.compact ? '' : (p.binance ? '' : p.head) + '****') + p.tail
})
const plain = computed(() => (mask.value && !parsed.value ? mask.value.masked : formatMaskedIBAN(props.id)))

const open = ref(false)
const copied = ref(false)
const toggle = async () => {
  if (!props.interactive) return
  open.value = !open.value
  if (open.value) {
    await writeClipboardText(props.id)
    copied.value = true // отклик: текст зеленеет и всплывает тост — в строку ничего не добавляется, вёрстка не прыгает
    pushToast({ kind: 'ok', title: 'Скопировано', ttl: 1500 })
    setTimeout(() => { copied.value = false }, 1200)
  }
}

const editing = ref(false)
const draft = ref('')
const inputRef = ref(null)
const startEdit = async () => {
  draft.value = mask.value && mask.value.custom ? mask.value.masked : ''
  editing.value = true
  await nextTick()
  inputRef.value && inputRef.value.focus()
}
// клавиши поля не должны всплывать до карточки: пробел там переключает выбор и съедается
const onKey = (e) => {
  if (e.key === 'Enter') { e.preventDefault(); commit() }
  else if (e.key === 'Escape') { e.preventDefault(); editing.value = false }
}
const commit = async () => {
  if (!editing.value) return
  editing.value = false
  try { await saveMask(props.id, draft.value) } catch (err) { console.error('Mask save error:', err) }
}
</script>

<template>
  <span class="inline-flex items-center gap-1.5 min-w-0 max-w-full">
    <template v-if="editing">
      <input
        ref="inputRef"
        v-model="draft"
        maxlength="40"
        class="ui-input !py-0.5 !px-1.5 !text-[11px] w-44"
        :placeholder="mask && !mask.custom ? mask.masked : 'Имя счёта'"
        @click.stop
        @keydown.stop="onKey"
        @blur="editing = false"
      />
      <button type="button" class="opacity-70 hover:opacity-100" title="Сохранить (пусто — вернуть автоматический вид)" @mousedown.prevent @click.stop="commit">
        <UiIcon name="check" size="sm" />
      </button>
    </template>
    <template v-else>
      <span
        class="inline-flex items-center gap-1 min-w-0"
        :class="[interactive && 'cursor-pointer', copied && 'text-emerald-300']"
        :title="interactive ? (open ? 'Свернуть' : 'Развернуть и скопировать') : undefined"
        @click.stop="toggle"
      >
        <template v-if="open">
          <span class="whitespace-nowrap tracking-wide text-[11px]">{{ id }}</span>
        </template>
        <template v-else-if="parsed">
          <template v-if="!parsed.binance">
            <svg v-if="parsed.country === 'UA'" viewBox="0 0 3 2" class="w-4 h-3 rounded-[2px] shrink-0" aria-label="UA"><rect width="3" height="1" fill="#0057b7" /><rect y="1" width="3" height="1" fill="#ffd700" /></svg>
            <span v-else class="shrink-0 leading-none" :title="parsed.country">{{ flagEmoji(parsed.country) }}</span>
            <span class="tracking-wide shrink-0">{{ parsed.check }}</span>
          </template>
          <img v-if="bank.logo && !failedLogos.has(bank.logo)" :src="bank.logo" :alt="bank.name" :title="bank.name" class="w-3.5 h-3.5 rounded-[3px] shrink-0 object-contain" @error="failedLogos.add(bank.logo)" />
          <svg v-else viewBox="0 0 16 16" class="w-3.5 h-3.5 shrink-0" role="img" :aria-label="bank.name"><title>{{ bank.name }}</title>
            <rect width="16" height="16" rx="3.5" :fill="bank.color" />
            <text x="8" y="11.2" text-anchor="middle" :font-size="bank.small ? 7.5 : 10" font-weight="700" font-family="monospace" fill="#020617">{{ bank.letter }}</text>
          </svg>
          <span class="tracking-wide truncate">{{ tailText }}</span>
        </template>
        <span v-else class="truncate tracking-wide">{{ plain }}</span>
      </span>
      <button
        v-if="interactive && isIban && !open"
        type="button"
        class="shrink-0 opacity-40 hover:opacity-100 transition-opacity"
        title="Своё имя счёта"
        @click.stop="startEdit"
      >
        <UiIcon name="edit" size="sm" />
      </button>
    </template>
  </span>
</template>
