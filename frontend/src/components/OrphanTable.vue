<script setup>
import { formatMaskedIBAN } from '../utils/formatters.js'
import UiIconButton from './ui/UiIconButton.vue'
import UiButton from './ui/UiButton.vue'

defineProps({
  orphanSignatures: { type: Array, required: true },
  users: { type: Array, required: true },
  draggedItem: { type: Object, default: null },
  activeSelectIndex: { type: Number, default: null },
  searchPersonQuery: { type: String, default: '' },
  isSignatureInCache: { type: Function, required: true }
})

const emit = defineEmits([
  'update:searchPersonQuery',
  'drag-start',
  'drag-end',
  'link-signature',
  'update:activeSelectIndex',
  'open-link-editor'
])
</script>

<template>
  <div class="space-y-3 pt-4">
    <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
      <div class="text-xs font-bold text-rose-400 uppercase tracking-wider flex items-center gap-2">
        <span>БЕСХОЗ</span>
        <span class="text-[9px] px-1.5 py-0.2 bg-rose-500/10 border border-rose-500/20 rounded text-rose-400 font-mono">
          {{ orphanSignatures.length }}
        </span>
      </div>

      <div class="w-full sm:w-64">
        <input
          :value="searchPersonQuery"
          @input="emit('update:searchPersonQuery', $event.target.value)"
          type="text"
          placeholder="Поиск..."
          class="ui-input !bg-slate-900"
        />
      </div>
    </div>

    <div class="bg-slate-900/60 border border-slate-800 rounded overflow-x-auto">
      <table class="w-full text-left border-collapse text-xs">
        <thead>
          <tr class="border-b border-slate-800 text-[10px] text-slate-500 uppercase tracking-wider bg-slate-950/50">
            <th class="p-3 w-10 text-center"></th>
            <th class="p-3">АЛИАСЫ</th>
            <th class="p-3">СЧЕТА</th>
            <th class="p-3 text-center">ТРАНЗАКЦИЙ</th>
            <th class="p-3 text-right">ДЕЙСТВИЕ</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-slate-800/60 font-mono">
          <tr
            v-for="(item, idx) in orphanSignatures"
            :key="item.id"
            draggable="true"
            @dragstart="emit('drag-start', $event, item)"
            @dragend="emit('drag-end')"
            class="transition-all select-none"
            :class="draggedItem && draggedItem.id === item.id 
              ? 'bg-slate-950/80 border-2 border-dashed border-slate-800' 
              : 'hover:bg-slate-800/40 cursor-grab active:cursor-grabbing'"
          >
            <template v-if="draggedItem && draggedItem.id === item.id">
              <td colspan="5" class="p-3 text-center text-slate-600 text-xs italic font-mono select-none">
                Здесь когда-то был {{ item.name }}...
              </td>
            </template>

            <template v-else>
              <td class="p-3 text-center text-slate-500 font-bold hover:text-emerald-400 transition-colors">
                <span class="inline-block tracking-tighter text-[11px] font-mono select-none">⠿</span>
              </td>

              <!-- СТОЛБЕЦ «АЛИАСЫ» С КАРАНДАШИКОМ -->
              <td class="p-3 font-bold text-slate-200">
                <div class="flex items-center gap-2">
                  <span>{{ item.name }}</span>
                  <UiIconButton
                    icon="edit"
                    tone="accent"
                    size="sm"
                    title="Открыть редактор связей"
                    @click.stop="emit('open-link-editor', item.name, '', item.accounts)"
                  />
                </div>
              </td>

              <!-- СТОЛБЕЦ «СЧЕТА» -->
              <td class="p-3 font-mono">
                <div v-if="item.ibansArray && item.ibansArray.length > 0" class="flex flex-wrap items-center gap-1.5">
                  <span
                    v-for="iban in item.ibansArray"
                    :key="iban"
                    class="text-[10px] px-2 py-0.5 rounded border border-slate-800 bg-slate-950 font-mono select-none transition-colors"
                    :class="isSignatureInCache(iban) ? 'text-slate-300' : 'text-amber-400 border-amber-500/30'"
                  >
                    💳 {{ formatMaskedIBAN(iban) }}
                  </span>
                </div>
                <span v-else class="text-slate-600 font-bold select-none">—</span>
              </td>

              <td class="p-3 text-center text-slate-400 font-bold">
                {{ item.txCount }}
              </td>

              <td class="p-3 text-right" @click.stop @mousedown.stop @touchstart.stop @dragstart.prevent>
                <div v-if="activeSelectIndex === idx" class="flex items-center justify-end gap-1">
                  <select
                    @change="emit('link-signature', users.find(u => u.id === $event.target.value), item)"
                    class="ui-select !w-auto !px-2 !py-1 !border-slate-700"
                  >
                    <option value="" disabled selected class="bg-slate-950 text-slate-400 py-1">Нет</option>
                    <option v-for="u in users" :key="u.id" :value="u.id" class="bg-slate-950 text-slate-200 py-1">{{ u.name }}</option>
                  </select>

                  <!-- ВЕКТОРНЫЙ КРЕСТИК СБРОСА ВЫБОРА -->
                  <UiIconButton icon="close" title="Отмена" @click="emit('update:activeSelectIndex', null)" />
                </div>

                <UiButton v-else variant="subtle" size="sm" @click="emit('update:activeSelectIndex', idx)">Привязать</UiButton>
              </td>
            </template>
          </tr>

          <tr v-if="orphanSignatures.length === 0">
            <td colspan="5" class="p-8 text-center text-slate-600 text-xs uppercase">
              Бесхозных сигнатур не найдено
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>