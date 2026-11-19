<script setup>
import { onMounted, ref, watch } from 'vue'
import WorkspaceCard from '../components/WorkspaceCard.vue'
import WorkspaceModal from '../components/WorkspaceModal.vue'
import UiIcon from '../components/ui/UiIcon.vue'
import UiEmpty from '../components/ui/UiEmpty.vue'
import UiLoadError from '../components/ui/UiLoadError.vue'
import { useWorkspaces } from '../composables/useWorkspaces.js'
import { pushRequestError } from '../composables/useToasts.js'

// Раздел «Дела»: рабочие пространства — сферы жизни и проектов (Glovo, семейный бюджет, айти-проект) карточками-полотнами.
// Справа внизу кнопка «+» открывает окно создания; на карточке по наведению — карандаш (правка) и урна (удаление в два клика). Пока полотно — только обложка и название; содержимое дел строится следующими шагами.
const props = defineProps({
  dataVersion: { type: Number, default: null } // версия данных бэкенда: растёт при любой записи — список перечитывается
})

const { list, error, load, create, update, remove } = useWorkspaces()
const open = ref(false)
const editing = ref(null) // дело в окне правки; null — окно создаёт новое
const busy = ref(false)

onMounted(load)
watch(() => props.dataVersion, load)

const openCreate = () => { editing.value = null; open.value = true }
const openEdit = (w) => { editing.value = w; open.value = true }

const onSubmit = async (payload) => {
  busy.value = true
  const w = editing.value
  try {
    if (w) await update(w.id, payload)
    else await create(payload)
    open.value = false
  } catch (e) {
    pushRequestError(w ? 'Не удалось сохранить дело' : 'Не удалось создать дело', e) // окно остаётся открытым: введённое не пропадает
  } finally {
    busy.value = false
  }
}

const onRemove = async (w) => {
  try {
    await remove(w.id)
  } catch (e) {
    pushRequestError('Не удалось удалить дело', e)
  }
}
</script>

<template>
  <div class="flex flex-col min-h-[calc(100vh-9rem)]">
    <UiLoadError v-if="error" :message="error" class="mb-3" @retry="load" />

    <div v-if="list.length" class="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-5 gap-3">
      <WorkspaceCard v-for="w in list" :key="w.id" :workspace="w" editable @edit="openEdit" @remove="onRemove" />
    </div>
    <UiEmpty v-else-if="!error">Пока нет дел. Нажми «+», чтобы создать первое</UiEmpty>

    <!-- кнопка прилипает к низу экрана и держится у правого края страницы -->
    <div class="sticky bottom-6 mt-auto pt-6 flex justify-end pointer-events-none">
      <button type="button" class="ws-fab pointer-events-auto" title="Новое дело" aria-label="Новое дело" @click="openCreate">
        <UiIcon name="plus" size="xl" />
      </button>
    </div>

    <WorkspaceModal :is-open="open" :workspace="editing" :busy="busy" @close="open = false" @submit="onSubmit" />
  </div>
</template>

<style scoped>
.ws-fab {
  width: 3rem;
  height: 3rem;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 9999px;
  background: rgb(16 185 129);
  color: rgb(2 6 23);
  box-shadow: 0 8px 20px rgb(0 0 0 / 0.45), 0 0 0 1px rgb(16 185 129 / 0.35);
  cursor: pointer;
  transition: background-color 0.15s, transform 0.15s, box-shadow 0.15s;
}
.ws-fab:hover { background: rgb(52 211 153); transform: scale(1.06); }
.ws-fab:active { transform: scale(0.96); }
.ws-fab:focus-visible { outline: 2px solid rgb(110 231 183); outline-offset: 3px; }
</style>
