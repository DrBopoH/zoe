<script setup>
import { ref, watch, inject } from 'vue'
import { API_BASE } from '../utils/api.js'
import UiModal from './ui/UiModal.vue'
import UiButton from './ui/UiButton.vue'

// Ручные даты открытия/закрытия счёта — костыль для банков, которые их не отдают.
// Хранятся в core.db (переживают снос кеша). Пустое поле = «считать по выписке».
const props = defineProps({
  account: { type: Object, default: null },
  isOpen: { type: Boolean, default: false }
})
const emit = defineEmits(['close'])
const refresh = inject('refreshData', null)

const toInput = (unix) => (unix ? new Date(unix * 1000).toISOString().slice(0, 10) : '')
const opened = ref('')
const closed = ref('')
const busy = ref(false)
const error = ref('')

watch(() => props.isOpen, (v) => {
  if (!v || !props.account) return
  // подставляем только то, что задано руками; автоматические даты остаются подсказкой в placeholder
  opened.value = props.account.manual_opened ? toInput(props.account.opened_at) : ''
  closed.value = props.account.manual_closed ? toInput(props.account.closed_at) : ''
  error.value = ''
})

const save = async () => {
  if (!props.account) return
  busy.value = true
  error.value = ''
  try {
    const res = await fetch(`${API_BASE}/accounts/life`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ account_id: props.account.id, opened: opened.value, closed: closed.value })
    })
    if (!res.ok) throw new Error((await res.text()) || res.status)
    if (refresh) await refresh()
    emit('close')
  } catch (e) {
    error.value = String(e.message || e)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <UiModal :is-open="isOpen" @close="emit('close')">
    <div class="p-5 space-y-4 font-mono">
      <div>
        <div class="text-xs font-bold uppercase tracking-wider text-slate-300">Срок жизни счёта</div>
        <div class="text-[10px] text-slate-500 mt-1 truncate">{{ account?.iban || account?.id }}</div>
      </div>

      <label class="block space-y-1">
        <span class="text-[10px] uppercase tracking-wider text-slate-500">Открыт</span>
        <input v-model="opened" type="date" class="ui-input w-full" />
        <span v-if="account && !account.manual_opened && account.opened_at" class="text-[10px] text-slate-600">
          по выписке: {{ toInput(account.opened_at) }}
        </span>
      </label>

      <label class="block space-y-1">
        <span class="text-[10px] uppercase tracking-wider text-slate-500">Закрыт</span>
        <input v-model="closed" type="date" class="ui-input w-full" />
        <span class="text-[10px] text-slate-600">пусто — счёт жив (или закроется сам, когда банк перестанет его отдавать)</span>
      </label>

      <div v-if="error" class="text-[11px] text-rose-400">{{ error }}</div>

      <div class="flex justify-end gap-2">
        <UiButton variant="cancel" size="sm" @click="emit('close')">Отмена</UiButton>
        <UiButton variant="primary" size="sm" :disabled="busy" @click="save">Сохранить</UiButton>
      </div>
    </div>
  </UiModal>
</template>
