<script setup>
defineProps({
  orphanSignatures: { type: Array, required: true },
  users: { type: Array, required: true },
  draggedItem: { type: Object, default: null },
  editingIbanItemId: { type: String, default: null },
  orphanIbanInputs: { type: Object, required: true },
  activeSelectIndex: { type: Number, default: null },
  searchPersonQuery: { type: String, default: '' },
  validateAndCleanAccount: { type: Function, required: true }
})

const emit = defineEmits([
  'update:searchPersonQuery',
  'drag-start',
  'drag-end',
  'paste-iban-save',
  'cancel-orphan-iban',
  'save-orphan-iban',
  'remove-orphan-iban',
  'start-edit-orphan-iban',
  'link-signature',
  'update:activeSelectIndex'
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
          class="w-full bg-slate-900 border border-slate-800 rounded px-3 py-1.5 text-xs text-white outline-none focus:border-emerald-500 font-mono"
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

              <td class="p-3 font-bold text-slate-200">
                {{ item.name }}
              </td>

              <!-- СТОЛБЕЦ «СЧЕТА» -->
              <td class="p-3 font-mono">
                <div v-if="editingIbanItemId === item.id" class="flex items-center gap-1.5" @click.stop @mousedown.stop>
                  <div class="relative flex items-center">
                    <input
                      v-model="orphanIbanInputs[item.id]"
                      type="text"
                      placeholder="Карта или IBAN..."
                      @contextmenu.prevent.stop
                      class="bg-slate-950 border text-xs rounded px-2.5 py-1 text-white outline-none font-mono pr-7 w-48 transition-colors"
                      :class="[
                        orphanIbanInputs[item.id] && !validateAndCleanAccount(orphanIbanInputs[item.id])
                          ? 'border-rose-500/80 focus:border-rose-400'
                          : 'border-slate-700 focus:border-emerald-500'
                      ]"
                    />
                    <button
                      type="button"
                      @mousedown.prevent.stop
                      @contextmenu.prevent.stop
                      @click.prevent.stop="emit('paste-iban-save', item, $event)"
                      class="absolute right-1 text-slate-400 hover:text-emerald-400 p-1 cursor-pointer transition-colors"
                      title="Хот-вставка из буфера с авто-сохранением"
                    >
                      <svg class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                        <path d="M16 4h2a2 2 0 012 2v14a2 2 0 01-2 2H6a2 2 0 01-2-2V6a2 2 0 012-2h2" />
                        <rect x="8" y="2" width="8" height="4" rx="1" ry="1" />
                      </svg>
                    </button>
                  </div>

                  <button
                    type="button"
                    @click.stop="emit('cancel-orphan-iban', item.id)"
                    class="bg-rose-600 hover:bg-rose-500 text-white text-[10px] uppercase font-bold px-2.5 py-1 rounded transition-all cursor-pointer shadow-sm"
                  >
                    Отмена
                  </button>

                  <button
                    type="button"
                    @click.stop="emit('save-orphan-iban', item)"
                    :disabled="!validateAndCleanAccount(orphanIbanInputs[item.id])"
                    class="text-[10px] uppercase font-bold px-2.5 py-1 rounded transition-all shadow-sm"
                    :class="[
                      !validateAndCleanAccount(orphanIbanInputs[item.id])
                        ? 'bg-slate-800 text-slate-500 border border-slate-700/60 cursor-not-allowed' 
                        : 'bg-emerald-600 hover:bg-emerald-500 text-white cursor-pointer'
                    ]"
                  >
                    Сохранить
                  </button>
                </div>

                <div v-else class="flex flex-wrap items-center gap-1.5">
                  <template v-if="item.ibansArray && item.ibansArray.length > 0">
                    <span
                      v-for="iban in item.ibansArray"
                      :key="iban"
                      class="text-[10px] px-2 py-0.5 rounded border border-slate-800 bg-slate-950 text-slate-300 font-mono flex items-center gap-1.5 transition-colors hover:border-slate-700 select-none"
                    >
                      <span>{{ iban }}</span>
                      <button
                        type="button"
                        @click.stop="emit('remove-orphan-iban', item.id, iban)"
                        class="text-slate-500 hover:text-rose-400 font-bold cursor-pointer transition-colors"
                        title="Стереть счет"
                      >
                        ✕
                      </button>
                    </span>
                  </template>

                  <button
                    type="button"
                    @click.stop="emit('start-edit-orphan-iban', item.id)"
                    class="text-slate-500 hover:text-emerald-400 hover:bg-slate-800 border border-slate-800 hover:border-slate-700 rounded w-5 h-5 flex items-center justify-center text-xs transition-all font-bold cursor-pointer shrink-0"
                    title="Добавить счет"
                  >
                    +
                  </button>
                </div>
              </td>

              <td class="p-3 text-center text-slate-400 font-bold">
                {{ item.txCount }}
              </td>

              <td class="p-3 text-right" @click.stop @mousedown.stop @touchstart.stop @dragstart.prevent>
                <div v-if="activeSelectIndex === idx" class="flex items-center justify-end gap-1">
                  <select
                    @change="emit('link-signature', users.find(u => u.id === $event.target.value), item)"
                    class="bg-slate-950 border border-slate-700 text-xs rounded px-2 py-1 text-white outline-none focus:border-emerald-500 font-mono"
                  >
                    <option value="" disabled selected>Выберите лицо...</option>
                    <option v-for="u in users" :key="u.id" :value="u.id">{{ u.name }}</option>
                  </select>
                  <button
                    @click="emit('update:activeSelectIndex', null)"
                    class="text-slate-500 hover:text-slate-300 text-[10px] uppercase px-1 cursor-pointer"
                  >
                    ✕
                  </button>
                </div>

                <button
                  v-else
                  @click="emit('update:activeSelectIndex', idx)"
                  class="bg-slate-800 hover:bg-slate-700 text-emerald-400 border border-slate-700 font-bold text-[10px] px-2.5 py-1 rounded transition-colors uppercase cursor-pointer"
                >
                  Привязать
                </button>
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