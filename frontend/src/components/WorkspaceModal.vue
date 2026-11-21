<script setup>
import { ref, computed, watch, nextTick } from 'vue'
import UiModal from './ui/UiModal.vue'
import UiButton from './ui/UiButton.vue'
import UiIconButton from './ui/UiIconButton.vue'
import WorkspaceCard from './WorkspaceCard.vue'
import { WORKSPACE_PAGES } from '../utils/workspacePages.js'
import { PALETTE, NAME_MAX, cleanName, nameOk, validColor, imageOk } from '../utils/workspaceCover.js'

// Окно создания и правки дела: название, цвет обложки (палитра или свой) и необязательная ссылка на картинку. Сверху живой предпросмотр той же карточки, что будет в «Делах».
const props = defineProps({
  isOpen: { type: Boolean, default: false },
  workspace: { type: Object, default: null }, // дело для правки; без него окно создаёт новое
  busy: { type: Boolean, default: false } // запрос на создание идёт
})
const emit = defineEmits(['close', 'submit'])

const name = ref('')
const color = ref(PALETTE[0])
const image = ref('')
const page = ref('')
const nameInput = ref(null)

watch(() => props.isOpen, (v) => {
  if (!v) return
  name.value = props.workspace?.name || ''
  color.value = validColor(props.workspace?.color) || PALETTE[0]
  image.value = props.workspace?.image || ''
  page.value = props.workspace?.page || ''
  nextTick(() => nameInput.value?.focus())
})

const linkBad = computed(() => !!image.value && !imageOk(image.value))
const can = computed(() => nameOk(name.value) && !linkBad.value && !props.busy)
const preview = computed(() => ({ name: cleanName(name.value) || 'Название дела', color: color.value, image: image.value }))
const imageText = computed({ get: () => image.value, set: (v) => { image.value = v.trim() } })

const onColor = (e) => { color.value = validColor(e.target.value) || color.value }

const submit = () => {
  if (!can.value) return
  emit('submit', { name: cleanName(name.value), color: color.value, image: image.value, page: page.value })
}
</script>

<template>
  <UiModal :is-open="isOpen" max-width="max-w-md" @close="emit('close')">
    <div class="flex items-center justify-between gap-3">
      <span class="text-sm font-bold text-white uppercase tracking-wider">{{ workspace ? 'Правка дела' : 'Новое дело' }}</span>
      <UiIconButton icon="close" title="Закрыть" @click="emit('close')" />
    </div>

    <div class="w-2/3 mx-auto"><WorkspaceCard :workspace="preview" /></div>

    <label class="block space-y-1">
      <span class="ui-label">Название</span>
      <input
        ref="nameInput"
        v-model="name"
        type="text"
        class="ui-input"
        :maxlength="NAME_MAX"
        placeholder="Например: Семейный бюджет"
        @keydown.enter.prevent="submit"
      />
    </label>

    <div>
      <div class="flex flex-wrap items-center gap-2">
        <button
          v-for="c in PALETTE"
          :key="c"
          type="button"
          class="ws-swatch"
          :class="color === c && 'ws-swatch--on'"
          :style="{ backgroundColor: c }"
          :aria-label="`Цвет ${c}`"
          @click="color = c"
        ></button>
        <label class="ws-swatch ws-swatch--own" :class="!PALETTE.includes(color) && 'ws-swatch--on'" :style="!PALETTE.includes(color) ? { backgroundColor: color } : null" title="Свой цвет">
          <input type="color" class="sr-only" :value="color" @input="onColor" />
        </label>
      </div>
    </div>

    <div class="space-y-1">
      <input v-model="imageText" type="text" class="ui-input" placeholder="Ссылка на картинку (https://…)" @keydown.enter.prevent="submit" />
      <p v-if="linkBad" class="text-[10px] text-rose-400">Нужна ссылка вида https://… без пробелов и скобок</p>
    </div>

    <label class="block space-y-1">
      <span class="ui-label">Страница</span>
      <select v-model="page" class="ui-input">
        <option value="">Пока без страницы</option>
        <option v-for="p in WORKSPACE_PAGES" :key="p.key" :value="p.key">{{ p.label }}</option>
      </select>
    </label>

    <div class="flex justify-end gap-2 pt-1">
      <UiButton variant="cancel" @click="emit('close')">Отмена</UiButton>
      <UiButton variant="primary" :disabled="!can" @click="submit">{{ busy ? (workspace ? 'Сохраняю…' : 'Создаю…') : (workspace ? 'Сохранить' : 'Создать') }}</UiButton>
    </div>
  </UiModal>
</template>

<style scoped>
.ws-swatch {
  width: 1.5rem;
  height: 1.5rem;
  border-radius: 9999px;
  border: 2px solid transparent;
  cursor: pointer;
  transition: transform 0.15s, border-color 0.15s;
}
.ws-swatch:hover { transform: scale(1.12); }
.ws-swatch--on { border-color: #fff; box-shadow: 0 0 0 2px rgb(2 6 23); }
/* «свой цвет»: радужная заливка, пока не выбран */
.ws-swatch--own { background-image: conic-gradient(#f43f5e, #eab308, #10b981, #0ea5e9, #a855f7, #f43f5e); display: inline-block; }
.ws-swatch--own.ws-swatch--on { background-image: none; }
</style>
