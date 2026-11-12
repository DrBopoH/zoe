<script setup>
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import UiButton from './ui/UiButton.vue'
import { parseIcuScreen, mergeEntries } from '../utils/icuOcr.js'
import { parseIcuList, ytmFromPrice } from '../utils/marketTable.js'

// Список бумаг брокера (ICU) для режима «только ICU»: скриншоты экрана брокера → распознавание → проверка → сохранение.
// Печатать ничего не нужно: картинки можно бросить в область, выбрать файлами или вставить (Ctrl+V). Распознавание идёт
// в браузере, картинки никуда не уходят. Ниже — кривая доходности (для бумаг без цены) и запасной ввод текстом.
const props = defineProps({
  registry: { type: Array, default: () => [] }, // бумаги реестра НБУ: по ним проверяются ISIN
  today: { type: String, required: true },
  icu: { type: Object, required: true } // useIcuList()
})
const emit = defineEmits(['close'])

const byId = computed(() => new Map(props.registry.map((b) => [String(b.id).toUpperCase(), b])))

// ── скриншоты ──
const rows = ref([]) // записи на проверке: { isin, maturity, kind, shown, ytm, price, flags, include }
const busy = ref('') // подпись хода работы
const err = ref('')
const mode = ref('add') // add — дополнить список, replace — заменить целиком
const dragOver = ref(false)
const fileInput = ref(null)
const done = ref(0)

const addImages = async (files) => {
  const imgs = [...files].filter((f) => f.type.startsWith('image/'))
  if (!imgs.length) return
  err.value = ''
  try {
    const { recognizeScreen } = await import('../utils/icuOcrRun.js')
    const parsed = rows.value.length ? [rows.value] : []
    for (let i = 0; i < imgs.length; i++) {
      busy.value = `Распознаю ${i + 1} из ${imgs.length}…`
      const texts = await recognizeScreen(imgs[i])
      parsed.push(mergeEntries(texts.map((t) => parseIcuScreen(t, props.registry, props.today))))
      done.value++
    }
    const merged = mergeEntries(parsed)
    const old = new Map(rows.value.map((r) => [r.isin, r]))
    rows.value = merged.map((e) => old.get(e.isin) || { ...e, include: true })
    if (!merged.length) err.value = 'На картинках не нашёл карточек с ISIN и доходностью (YTM). Нужен экран «Купити → ОВДП».'
  } catch (e) {
    err.value = `Не удалось распознать: ${e.message || e}`
  } finally {
    busy.value = ''
  }
}
const onDrop = (e) => { dragOver.value = false; addImages(e.dataTransfer?.files || []) }
const onPick = (e) => { addImages(e.target.files || []); e.target.value = '' }
const onPaste = (e) => {
  const files = [...(e.clipboardData?.files || [])]
  if (files.some((f) => f.type.startsWith('image/'))) { e.preventDefault(); addImages(files) }
}
onMounted(() => window.addEventListener('paste', onPaste))
onBeforeUnmount(() => window.removeEventListener('paste', onPaste))

const calc = (r) => {
  const b = byId.value.get(r.isin)
  return b && r.price > 0 ? ytmFromPrice(b, r.price, props.today) : null
}
const fmt = (v) => (v == null ? '—' : (Math.round(v * 100) / 100).toFixed(2).replace(/0$/, '').replace(/\.$/, '').replace('.', ','))
const FLAG_TEXT = {
  'isin-fixed': 'ISIN исправлен по реестру',
  'isin-ambiguous': 'ISIN неоднозначен, проверь',
  'not-in-registry': 'нет в реестре НБУ',
  'price-none': 'цену не прочитал',
  'price-bad': 'цена не сошлась с YTM'
}
const soon = (r) => !!r.maturity && (Date.parse(`${r.maturity}T00:00:00Z`) - Date.parse(`${props.today}T00:00:00Z`)) / 86400000 < 30
const notes = (r) => [...r.flags.map((f) => FLAG_TEXT[f] || f), ...(soon(r) ? ['гасится меньше чем через месяц: годовая доходность условна'] : [])]
const bad = (r) => r.flags.includes('not-in-registry') || r.flags.includes('isin-ambiguous')
const included = computed(() => rows.value.filter((r) => r.include))
const dateText = (iso) => (iso ? iso.split('-').reverse().join('.') : '—')

const saveShots = async () => {
  try {
    const ids = mode.value === 'replace' ? [] : [...props.icu.isins.value]
    const prices = mode.value === 'replace' ? {} : { ...props.icu.prices.value }
    const ytms = mode.value === 'replace' ? {} : { ...props.icu.ytms.value }
    for (const r of included.value) {
      if (!ids.includes(r.isin)) ids.push(r.isin)
      if (r.price > 0) prices[r.isin] = r.price; else delete prices[r.isin]
      if (r.ytm > 0) ytms[r.isin] = Math.round(r.ytm * 100) / 100; else delete ytms[r.isin]
    }
    await props.icu.save({ isins: ids, prices, ytms })
    rows.value = []
    err.value = ''
  } catch (e) {
    err.value = String(e.message || e)
  }
}

// ── кривая доходности ──
const pts = ref((props.icu.curve.value || []).map((p) => ({ years: p.years, ytm: p.ytm })))
const addPoint = () => pts.value.push({ years: '', ytm: '' })
const saveCurve = async () => {
  try {
    await props.icu.saveCurve(pts.value.map((p) => ({ years: Number(String(p.years).replace(',', '.')), ytm: Number(String(p.ytm).replace(',', '.')) })))
    pts.value = props.icu.curve.value.map((p) => ({ years: p.years, ytm: p.ytm }))
    err.value = ''
  } catch (e) {
    err.value = String(e.message || e)
  }
}

// ── запасной ввод текстом ──
const textOpen = ref(false)
const text = ref('')
const openText = () => {
  text.value = props.icu.isins.value.map((id) => (props.icu.prices.value[id] ? `${id} ${props.icu.prices.value[id]}` : id)).join('\n')
  textOpen.value = !textOpen.value
}
const saveText = async () => {
  try {
    const p = parseIcuList(text.value)
    await props.icu.save({ isins: p.isins, prices: p.prices, ytms: Object.fromEntries(p.isins.filter((i) => props.icu.ytms.value[i] && !p.prices[i]).map((i) => [i, props.icu.ytms.value[i]])) })
    textOpen.value = false
    err.value = ''
  } catch (e) {
    err.value = String(e.message || e)
  }
}
const parsedText = computed(() => parseIcuList(text.value))
const withPrice = computed(() => Object.keys(props.icu.prices.value).length)
</script>

<template>
  <section class="bg-slate-900/60 border border-slate-800 rounded p-4 space-y-4 text-xs">
    <div class="flex items-center justify-between gap-3">
      <div class="font-bold text-slate-300 uppercase tracking-wider">Список ICU</div>
      <div class="flex items-center gap-4 text-slate-500">
        <span>в списке {{ icu.isins.value.length }}, с ценой {{ withPrice }}</span>
        <button type="button" class="hover:text-slate-300 underline cursor-pointer" @click="emit('close')">закрыть</button>
      </div>
    </div>

    <div
      class="border-2 border-dashed rounded p-5 text-center cursor-pointer transition-colors"
      :class="dragOver ? 'border-emerald-500/70 bg-emerald-500/5 text-emerald-300' : 'border-slate-700 text-slate-400 hover:border-slate-500'"
      @dragover.prevent="dragOver = true"
      @dragleave="dragOver = false"
      @drop.prevent="onDrop"
      @click="fileInput.click()"
    >
      <template v-if="busy">{{ busy }}</template>
      <template v-else>Брось сюда скриншоты экрана ICU «Купити → ОВДП» (можно несколько), выбери файлы или нажми Ctrl+V с картинкой</template>
      <input ref="fileInput" type="file" accept="image/*" multiple class="hidden" @change="onPick" />
    </div>

    <div v-if="rows.length" class="space-y-2">
      <div class="overflow-x-auto">
        <table class="w-full text-[11px]">
          <thead>
            <tr class="text-slate-500 uppercase tracking-wider text-left">
              <th class="py-1 pr-2 w-6"></th><th class="pr-3">ISIN</th><th class="pr-3">Погашение</th><th class="pr-3 text-right">На экране</th>
              <th class="pr-3 text-right">Цена, ₴</th><th class="pr-3 text-right">Доходность по цене</th><th>Проверка</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="r in rows" :key="r.isin" class="border-t border-slate-800" :class="{ 'opacity-40': !r.include }">
              <td class="py-1 pr-2"><input v-model="r.include" type="checkbox" class="accent-emerald-500" /></td>
              <td class="pr-3 font-mono text-slate-300">{{ r.isin }}</td>
              <td class="pr-3 text-slate-400">{{ dateText(r.maturity) }}</td>
              <td class="pr-3 text-right text-slate-300">{{ r.kind }} {{ fmt(r.shown) }} %</td>
              <td class="pr-3 text-right"><input v-model.number="r.price" type="number" step="0.01" class="ui-input !bg-slate-900 !w-24 text-right !py-0.5" placeholder="—" /></td>
              <td class="pr-3 text-right text-slate-300">{{ calc(r) == null ? '—' : fmt(calc(r)) + ' %' }}</td>
              <td :class="bad(r) ? 'text-rose-400' : notes(r).length ? 'text-amber-300' : 'text-emerald-400'">{{ notes(r).length ? notes(r).join('; ') : 'ок' }}</td>
            </tr>
          </tbody>
        </table>
      </div>
      <div class="flex flex-wrap items-center gap-4">
        <label class="flex items-center gap-1.5 text-slate-400 cursor-pointer"><input v-model="mode" type="radio" value="add" class="accent-emerald-500" />дополнить список</label>
        <label class="flex items-center gap-1.5 text-slate-400 cursor-pointer" title="Бумаг, которых нет на этих скриншотах, в списке не останется"><input v-model="mode" type="radio" value="replace" class="accent-emerald-500" />заменить весь список</label>
        <UiButton variant="secondary" size="sm" :disabled="!included.length" @click="saveShots">СОХРАНИТЬ ({{ included.length }})</UiButton>
        <button type="button" class="text-slate-500 hover:text-slate-300 underline cursor-pointer" @click="rows = []">сбросить</button>
        <span class="text-slate-600">Цену можно поправить руками; если она не сходится с YTM, берётся показанная доходность.</span>
      </div>
    </div>

    <div class="space-y-2 border-t border-slate-800 pt-3">
      <div class="text-slate-400">Кривая доходности: срок в годах → доходность, %. Ею оцениваются бумаги без цены (вместе с бумагами, у которых доходность известна). Три–пять точек с экрана ICU достаточно.</div>
      <div class="flex flex-wrap items-center gap-x-4 gap-y-2">
        <span v-for="(p, i) in pts" :key="i" class="inline-flex items-center gap-1">
          <input v-model="p.years" inputmode="decimal" class="ui-input !bg-slate-900 !w-16 text-right !py-0.5" placeholder="лет" />
          <input v-model="p.ytm" inputmode="decimal" class="ui-input !bg-slate-900 !w-16 text-right !py-0.5" placeholder="%" />
          <button type="button" class="text-slate-600 hover:text-rose-400 cursor-pointer" title="убрать точку" @click="pts.splice(i, 1)">×</button>
        </span>
        <button type="button" class="text-slate-500 hover:text-emerald-400 underline cursor-pointer" @click="addPoint">+ точка</button>
        <UiButton variant="secondary" size="sm" @click="saveCurve">СОХРАНИТЬ КРИВУЮ</UiButton>
      </div>
    </div>

    <div class="border-t border-slate-800 pt-3 space-y-2">
      <button type="button" class="text-slate-500 hover:text-slate-300 underline cursor-pointer" @click="openText">{{ textOpen ? 'скрыть ввод текстом' : 'вставить текстом' }}</button>
      <template v-if="textOpen">
        <textarea v-model="text" rows="4" class="ui-input !bg-slate-900 w-full font-mono" placeholder="UA4000239040 1011,12&#10;UA4000239008 1067,95&#10;…"></textarea>
        <div class="flex items-center gap-3">
          <UiButton variant="secondary" size="sm" @click="saveText">СОХРАНИТЬ</UiButton>
          <span class="text-slate-500">найдено ISIN: {{ parsedText.isins.length }}, с ценой: {{ Object.keys(parsedText.prices).length }}</span>
        </div>
      </template>
    </div>

    <div v-if="err || icu.error.value" class="text-rose-400">{{ err || icu.error.value }}</div>
  </section>
</template>
