<script setup>
import { ref, inject, computed } from 'vue'
import { apiGetJson, apiSend, q } from '../utils/api.js'
import UiButton from './ui/UiButton.vue'
import UiIconButton from './ui/UiIconButton.vue'
import { parseDateText, isoToText } from '../utils/dateText.js'

// Справочник облигаций вместо ввода карт: плитка на каждую ОВДП (ISIN, погашение, ставка, номинал).
// В брокерском отчёте этих данных нет; один справочник на всех счетах. Пишется сразу (не ждёт «Сохранить» алиаса).
// Покупки и продажи берутся из отчёта.
const refresh = inject('refreshData', null)

const bonds = ref([])
const form = ref(null)
const error = ref('')
const busy = ref(false)

const load = async () => {
  try {
    bonds.value = await apiGetJson('/icu/bonds')
    error.value = ''
  } catch (e) {
    error.value = String(e.message || e)
  }
}
load()

const blank = () => ({ isin: '', name: '', maturity: '', coupon_rate: null, nominal: 1000 })
// дата в форме — строкой ДД.ММ.ГГГГ; название и номинал (всегда 1000) не спрашиваем
const editing = ref(null) // ISIN правимой облигации; null при создании новой
const edit = (b) => { editing.value = b ? b.isin : null; form.value = b ? { ...b, maturity: isoToText(b.maturity) } : blank() }
// форма открывается на месте плитки, которую правим; новая — в конце списка
const rows = computed(() => (form.value && editing.value === null ? [...bonds.value, { isin: '', isNew: true }] : bonds.value))
const showForm = (r) => form.value && (r.isNew || r.isin === editing.value)

const ok = computed(() => form.value && form.value.isin.trim() && parseDateText(form.value.maturity) && Number(form.value.coupon_rate) > 0)

const run = async (fn) => {
  busy.value = true
  error.value = ''
  try {
    await fn()
    await load()
    if (refresh) await refresh()
  } catch (e) {
    error.value = String(e.message || e)
  } finally {
    busy.value = false
  }
}
const save = () => run(async () => {
  const f = form.value
  await apiSend('POST', '/icu/bonds', {
    isin: f.isin.trim().toUpperCase(), name: f.name || '',
    maturity: parseDateText(f.maturity), coupon_rate: Number(f.coupon_rate), nominal: f.nominal || 1000
  })
  form.value = null
})
const remove = (b) => run(() => apiSend('DELETE', `/icu/bonds?isin=${q(b.isin)}`))

const fmtDate = (iso) => iso.split('-').reverse().join('.')
</script>

<template>
  <div class="ml-5 space-y-2 border-l border-blue-500/30 pl-3">
    <div class="flex items-center justify-between">
      <span class="ui-label">Справочник облигаций</span>
      <UiButton variant="subtle" size="sm" @click="edit(null)">+ ОВДП</UiButton>
    </div>

    <template v-for="b in rows" :key="b.isin || '__new'">
      <div v-if="showForm(b)" class="space-y-2 rounded border border-slate-800 bg-slate-950/40 p-2">
        <div class="grid grid-cols-3 gap-2">
          <label class="space-y-1"><span class="ui-label">ISIN</span><input v-model="form.isin" class="ui-input" placeholder="UA4000239081" /></label>
          <label class="space-y-1"><span class="ui-label">Погашение</span><input v-model="form.maturity" inputmode="numeric" class="ui-input" :class="form.maturity && !parseDateText(form.maturity) && 'ui-input--invalid'" placeholder="ДД.ММ.ГГГГ" @keyup.enter="ok && save()" /></label>
          <label class="space-y-1"><span class="ui-label">Ставка, %</span><input v-model.number="form.coupon_rate" type="number" step="0.01" class="ui-input" placeholder="16.44" @keyup.enter="ok && save()" /></label>
        </div>
        <div class="text-[10px] text-slate-600">номинал 1000 ₴; купон раз в 182 дня назад от погашения</div>
        <div class="flex justify-end gap-2">
          <UiButton variant="cancel" size="sm" @click="form = null">Отмена</UiButton>
          <UiButton variant="primary" size="sm" :disabled="busy || !ok" @click="save">Сохранить</UiButton>
        </div>
      </div>

      <div v-else class="flex items-center gap-2 text-[11px] rounded border border-blue-500/30 bg-blue-950/30 px-2 py-1.5">
        <div class="flex-1 min-w-0">
          <div class="font-bold text-blue-100 truncate">{{ b.isin }} <span class="font-normal text-slate-400">{{ b.name }}</span></div>
          <div class="text-[10px] text-slate-400">до {{ fmtDate(b.maturity) }} · {{ b.coupon_rate }}% · номинал {{ b.nominal }} ₴</div>
        </div>
        <UiIconButton icon="edit" tone="accent" size="sm" @click="edit(b)" />
        <UiIconButton icon="trash" tone="danger" size="sm" @click="remove(b)" />
      </div>
    </template>
    <div v-if="!bonds.length && !form" class="text-[10px] text-slate-600 italic">Облигаций пока нет</div>

    <div v-if="error" class="text-[11px] text-rose-400">{{ error }}</div>
  </div>
</template>
