<script setup>
import { computed, ref } from 'vue'
import { formatMaskedIBAN, lifeSpan } from '../utils/formatters.js'
import { buildPersonLayout } from '../utils/personLayout.js'
import { bankTheme } from '../utils/bankTheme.js'
import { readClipboardText } from '../utils/clipboard.js'
import UiIcon from './ui/UiIcon.vue'
import UiIconButton from './ui/UiIconButton.vue'
import UiButton from './ui/UiButton.vue'
import UiDot from './ui/UiDot.vue'
import UiAliasChip from './ui/UiAliasChip.vue'
import AccountLifeModal from './AccountLifeModal.vue'

const props = defineProps({
  user: { type: Object, required: true },
  index: { type: Number, required: true },
  draggedItem: { type: Object, default: null },
  draggedCardIndex: { type: Number, default: null },
  activeHoverUser: { type: [String, Number], default: null },
  editingUserId: { type: [String, Number], default: null },
  editForm: { type: Object, required: true },
  activeNamePasteUsers: { type: Object, required: true },
  expandedIbans: { type: Object, required: true },
  copiedIban: { type: String, default: '' },
  canSave: { type: Function, required: true },
  isSignatureInCache: { type: Function, required: true },
  graph: { type: Object, default: null }
})

const lifeAcc = ref(null) // счёт, у которого правят даты открытия/закрытия

const emit = defineEmits([
  'card-dragstart',
  'card-dragend',
  'dragover-card',
  'dragleave-card',
  'mouseleave-card',
  'drop-user',
  'cancel-edit',
  'save-user',
  'paste-avatar',
  'mouseenter-name',
  'mouseleave-name',
  'paste-name',
  'edit-user',
  'delete-user',
  'unlink-alias',
  'click-iban',
  'open-link-editor'
])

const TILES_PER_ROW = 3
const chunkTiles = (tiles) => {
  const rows = []
  for (let i = 0; i < tiles.length; i += TILES_PER_ROW) rows.push(tiles.slice(i, i + TILES_PER_ROW))
  return rows
}

const layout = computed(() => buildPersonLayout(props.user, props.graph))

const preparedGroups = computed(() =>
  layout.value.groups.map(group => ({
    ...group,
    mainAliases: group.aliases.filter(a => !a.isSub),
    subAliases: group.aliases.filter(a => a.isSub),
    tiles: group.tiles.map(t => ({ ...t, theme: bankTheme(t.bankKey, t.type) }))
  }))
)

const isEditing = computed(() => String(props.editingUserId) === String(props.user.id) || !props.user.name || props.user.isDraft)
const isHovered = computed(() => String(props.activeHoverUser) === String(props.user.id))

// Чтение буфера — первым действием прямо в обработчике клика (см. utils/clipboard.js)
const pasteFromClipboard = async (eventName) => {
  try {
    const text = await readClipboardText()
    if (text) emit(eventName, props.user, text)
  } catch (err) {
    console.error('Clipboard read error:', err)
  }
}
const onPasteAvatar = () => pasteFromClipboard('paste-avatar')
const onPasteName = () => pasteFromClipboard('paste-name')
</script>

<template>
  <div
    :draggable="!isEditing"
    @dragstart="emit('card-dragstart', $event, index)"
    @dragend="emit('card-dragend')"
    @dragover="emit('dragover-card', $event, user.id)"
    @dragenter.prevent
    @dragleave="emit('dragleave-card', $event, user.id)"
    @mouseleave="emit('mouseleave-card', user.id)"
    @drop.prevent="emit('drop-user', index, user)"
    class="bg-slate-900/70 border p-4 rounded-lg transition-all duration-300 flex flex-col justify-between min-h-[170px] relative group overflow-hidden select-none cursor-grab active:cursor-grabbing"
    :class="[
      isHovered
        ? 'border-emerald-400 bg-slate-900 scale-[1.02] shadow-lg shadow-emerald-500/20'
        : (draggedItem || draggedCardIndex !== null)
          ? 'border-emerald-500/60 neon-border-pulse bg-emerald-950/10'
          : 'border-slate-800'
    ]"
  >
    <!-- 1. ПРИВЯЗКА ПРИ ДРАГЕ ИЗ БЕСХОЗА -->
    <Transition name="fade">
      <div
        v-if="draggedItem && isHovered"
        class="absolute inset-0 bg-slate-950/90 backdrop-blur-sm z-10 flex flex-col items-center justify-start pt-3 pointer-events-none"
      >
        <div class="text-[10px] font-bold text-emerald-400 uppercase tracking-wider font-mono">
          ПРИВЯЗАТЬ К ЛИЦУ
        </div>
      </div>
    </Transition>

    <!-- 2. ИНЛАЙН РЕДАКТИРОВАНИЕ / СОЗДАНИЕ -->
    <template v-if="isEditing">
      <div class="space-y-2 relative z-20 cursor-default" @mousedown.stop>
        <input v-model="editForm.name" type="text" placeholder="Имя..." class="ui-input" />
        <input v-model="editForm.avatar_url" type="text" placeholder="URL Аватара..." class="ui-input" />

        <div class="flex items-center justify-between pt-1">
          <UiButton variant="danger" size="sm" @click="emit('cancel-edit', user)">Отмена</UiButton>
          <UiButton variant="primary" size="sm" :disabled="!canSave(user)" @click="emit('save-user', user)">Сохранить</UiButton>
        </div>
      </div>
    </template>

    <!-- 3. ПРОСМОТР ПРОФИЛЯ -->
    <template v-else>
      <div class="space-y-3">
        <!-- Шапка: Аватар + Имя Лица -->
        <div
          class="relative z-20 flex items-center justify-between gap-3 transition-transform duration-300 ease-out"
          :class="draggedItem && isHovered ? 'translate-y-4 translate-x-27' : 'translate-y-0 translate-x-0'"
        >
          <div class="flex items-center gap-2.5 min-w-0">
            <div
              @mousedown.stop
              @contextmenu.stop
              @click.stop="onPasteAvatar"
              class="relative w-9 h-9 rounded-full bg-slate-800 border border-slate-700 overflow-hidden shrink-0 flex items-center justify-center font-bold text-slate-400 text-xs select-none group/avatar cursor-pointer"
              title="Вставить URL аватара из буфера обмена"
            >
              <img v-if="user.avatar_url" :src="user.avatar_url" class="w-full h-full object-cover" draggable="false" />
              <span v-else>{{ user.name.charAt(0).toUpperCase() }}</span>

              <div class="absolute inset-0 bg-slate-950/75 backdrop-blur-[1px] opacity-0 group-hover/avatar:opacity-100 flex items-center justify-center transition-opacity duration-300 text-slate-300 hover:text-emerald-400">
                <UiIcon name="paste" />
              </div>
            </div>

            <div
              class="flex items-center gap-1.5 min-w-0"
              @mouseenter="emit('mouseenter-name', user.id)"
              @mouseleave="emit('mouseleave-name', user.id)"
            >
              <span class="text-sm font-bold text-white truncate cursor-default">{{ user.name }}</span>

              <Transition name="slide-left">
                <UiIconButton
                  v-if="activeNamePasteUsers.has(user.id)"
                  icon="paste"
                  tone="accent"
                  size="sm"
                  title="Вставить имя из буфера обмена"
                  @mousedown.stop
                  @click.stop="onPasteName"
                />
              </Transition>
            </div>
          </div>

          <div class="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity" @mousedown.stop>
            <UiIconButton icon="edit" tone="accent" title="Редактировать лицо" @click="emit('edit-user', user)" />
            <UiIconButton icon="trash" tone="danger" title="Удалить лицо" @click="emit('delete-user', user.id)" />
          </div>
        </div>

        <!-- ЗАГУЛЯВШИЕ АЛИАСЫ (без привязанных счетов) -->
        <div v-if="layout.looseAliases.length > 0" class="flex flex-wrap items-center gap-1 font-mono cursor-default pt-1" @mousedown.stop>
          <UiAliasChip
            v-for="item in layout.looseAliases"
            :key="item.alias"
            :name="item.alias"
            :cached="isSignatureInCache(item.alias)"
            loose
            @edit="emit('open-link-editor', item.alias, user.id, item.accounts)"
            @unlink="emit('unlink-alias', user, item.alias)"
          />
        </div>
      </div>

      <!-- БАНКОВСКИЕ БЛОКИ -->
      <div
        v-if="preparedGroups.length > 0"
        class="space-y-3.5 cursor-default font-mono"
        :class="layout.looseAliases.length > 0 ? 'mt-5' : 'mt-3'"
        @mousedown.stop
      >
        <div v-for="group in preparedGroups" :key="group.key" class="space-y-1.5">
          <!-- АЛИАСЫ БАНКА: главные (владельцы счетов) в ряд, «другие» — столбиком под ними с └─ -->
          <div v-if="group.aliases.length > 0" class="space-y-1">
            <div v-if="group.mainAliases.length > 0" class="flex flex-wrap items-center gap-1">
              <UiAliasChip
                v-for="item in group.mainAliases"
                :key="item.alias"
                :name="item.alias"
                :cached="isSignatureInCache(item.alias)"
                @edit="emit('open-link-editor', item.alias, user.id, item.accounts)"
                @unlink="emit('unlink-alias', user, item.alias)"
              />
            </div>

            <div v-if="group.subAliases.length > 0" class="flex flex-col items-start gap-1 pl-3">
              <UiAliasChip
                v-for="item in group.subAliases"
                :key="item.alias"
                :name="item.alias"
                :cached="isSignatureInCache(item.alias)"
                sub
                @edit="emit('open-link-editor', item.alias, user.id, item.accounts)"
                @unlink="emit('unlink-alias', user, item.alias)"
              />
            </div>
          </div>

          <!-- РЯД КАРТ-ПЛИТОК БАНКА -->
          <!-- не больше 3 плиток в ряду; ряд прокручивается, если не влезает; 4-я и дальше — в новый ряд (плитка лица растёт вниз) -->
          <div v-if="group.tiles.length > 0" class="space-y-2">
          <div v-for="(row, rowIdx) in chunkTiles(group.tiles)" :key="rowIdx" class="flex gap-2 overflow-x-auto pb-2">
            <div
              v-for="tile in row"
              :key="tile.key"
              class="w-[145px] aspect-[85.6/54] rounded-lg p-2.5 border shadow-md flex flex-col justify-between relative overflow-hidden shrink-0"
              :class="[tile.theme.tile, tile.cacheAcc?.is_closed && 'ui-closed']"
              :title="lifeSpan(tile.cacheAcc) || undefined"
            >
              <button
                v-if="tile.cacheAcc"
                type="button"
                class="ui-life-btn"
                title="Даты открытия / закрытия"
                @click.stop="lifeAcc = tile.cacheAcc"
              >
                <UiIcon name="edit" size="sm" />
              </button>
              <!-- ШАПКА: IBAN -->
              <div class="flex items-start justify-between gap-1">
                <div
                  v-if="tile.iban"
                  @click="emit('click-iban', tile.iban)"
                  class="flex items-center gap-1 min-w-0 text-[9px] font-bold cursor-pointer select-none"
                >
                  <UiDot :cached="isSignatureInCache(tile.iban)" />
                  <span class="tracking-wide" :class="expandedIbans.has(tile.iban) ? 'break-all' : 'truncate'">
                    {{ expandedIbans.has(tile.iban) ? tile.iban : formatMaskedIBAN(tile.iban) }}
                  </span>
                  <span v-if="copiedIban === tile.iban" class="text-emerald-400 shrink-0">✓</span>
                </div>
                <span v-else></span>

                <span v-if="tile.cacheAcc?.is_closed" class="ui-closed-tag shrink-0">Закр.</span>
                <span v-else-if="tile.theme.tag" class="text-[8px] font-bold tracking-widest opacity-60 shrink-0">
                  {{ tile.theme.tag }}
                </span>
              </div>

              <!-- ЧИП -->
              <div class="w-5 h-3.5 rounded-[3px] bg-gradient-to-br from-yellow-200 to-yellow-600 opacity-70 shrink-0"></div>

              <!-- НИЗ: ГЛАВНЫЙ PAN И ДОЧЕРНИЕ -->
              <div class="space-y-1 min-h-[14px]">
                <div
                  v-if="tile.mainPan"
                  @click="emit('click-iban', tile.mainPan)"
                  class="text-[11px] font-semibold tracking-wider cursor-pointer select-none"
                  :class="expandedIbans.has(tile.mainPan) ? 'break-all' : 'truncate'"
                >
                  {{ expandedIbans.has(tile.mainPan) ? tile.mainPan : formatMaskedIBAN(tile.mainPan) }}
                  <span v-if="copiedIban === tile.mainPan" class="text-emerald-400 text-[9px] font-bold">✓</span>
                </div>

                <div v-if="tile.otherPans.length > 0" class="flex flex-wrap items-center gap-1 text-[8px]">
                  <span class="opacity-40 font-bold select-none">└─</span>
                  <span
                    v-for="otherPan in tile.otherPans"
                    :key="otherPan"
                    @click="emit('click-iban', otherPan)"
                    class="px-1 py-px rounded border border-white/15 bg-black/20 cursor-pointer select-none hover:border-white/40 transition-colors truncate max-w-full"
                  >
                    {{ expandedIbans.has(otherPan) ? otherPan : formatMaskedIBAN(otherPan) }}
                    <span v-if="copiedIban === otherPan" class="text-emerald-400 font-bold ml-0.5">✓</span>
                  </span>
                </div>
              </div>
            </div>
          </div>
          </div>
        </div>
      </div>

      <div
        v-if="layout.looseAliases.length === 0 && preparedGroups.length === 0"
        class="mt-3 text-[10px] text-slate-600 italic font-mono"
      >
        Перетащите контрагента...
      </div>
    </template>

    <AccountLifeModal :account="lifeAcc" :is-open="!!lifeAcc" @close="lifeAcc = null" />
  </div>
</template>
