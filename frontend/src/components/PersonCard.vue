<script setup>
defineProps({
  user: { type: Object, required: true },
  index: { type: Number, required: true },
  draggedItem: { type: Object, default: null },
  draggedCardIndex: { type: Number, default: null },
  activeHoverUser: { type: String, default: null },
  editingUserId: { type: String, default: null },
  editForm: { type: Object, required: true },
  activeNamePasteUsers: { type: Object, required: true },
  expandedIbans: { type: Object, required: true },
  copiedIban: { type: String, default: '' },
  canSave: { type: Function, required: true },
  isSignatureInCache: { type: Function, required: true },
  formatMaskedIBAN: { type: Function, required: true }
})

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
  'click-iban'
])
</script>

<template>
  <div
    :draggable="editingUserId !== user.id && !user.isDraft"
    @dragstart="emit('card-dragstart', $event, index)"
    @dragend="emit('card-dragend')"
    @dragover="emit('dragover-card', $event, user.id)"
    @dragenter.prevent
    @dragleave="emit('dragleave-card', $event, user.id)"
    @mouseleave="emit('mouseleave-card', user.id)"
    @drop.prevent="emit('drop-user', index, user)"
    class="bg-slate-900/70 border p-4 rounded-lg transition-all duration-300 flex flex-col justify-start min-h-[140px] relative group overflow-hidden select-none cursor-grab active:cursor-grabbing"
    :class="[
      activeHoverUser === user.id 
        ? 'border-emerald-400 bg-slate-900 scale-[1.02] shadow-lg shadow-emerald-500/20' 
        : (draggedItem || draggedCardIndex !== null)
          ? 'border-emerald-500/60 neon-border-pulse bg-emerald-950/10' 
          : 'border-slate-800'
    ]"
  >
    <!-- 1. ПОМУТНЕВШИЙ МАТОВЫЙ ОВЕРЛЕЙ ПРИ ДРАГЕ ИЗ БЕСХОЗА -->
    <Transition name="fade">
      <div
        v-if="draggedItem && activeHoverUser === user.id"
        class="absolute inset-0 bg-slate-950/90 backdrop-blur-sm z-10 flex flex-col items-center justify-start pt-3 pointer-events-none"
      >
        <div class="text-[10px] font-bold text-emerald-400 uppercase tracking-wider font-mono">
          ПРИВЯЗАТЬ К ЛИЦУ
        </div>
      </div>
    </Transition>

    <!-- 2. ИНЛАЙН РЕДАКТИРОВАНИЕ / СОЗДАНИЕ -->
    <template v-if="editingUserId === user.id || !user.name || user.isDraft">
      <div class="space-y-2 relative z-20 cursor-default" @mousedown.stop>
        <input
          v-model="editForm.name"
          type="text"
          placeholder="Имя..."
          class="w-full bg-slate-950 border border-slate-800 text-xs px-2.5 py-1.5 rounded text-white outline-none focus:border-emerald-500 font-mono"
        />
        <input
          v-model="editForm.avatar_url"
          type="text"
          placeholder="URL Аватара..."
          class="w-full bg-slate-950 border border-slate-800 text-xs px-2.5 py-1.5 rounded text-white outline-none focus:border-emerald-500 font-mono"
        />

        <!-- КНОПКИ ДЕЙСТВИЙ -->
        <div class="flex items-center justify-between pt-1">
          <button
            @click="emit('cancel-edit', user)"
            class="bg-rose-600 hover:bg-rose-500 text-white text-[10px] uppercase font-bold px-3 py-1 rounded transition-all cursor-pointer shadow-sm"
          >
            Отмена
          </button>

          <button
            @click="emit('save-user', user)"
            :disabled="!canSave(user)"
            class="text-[10px] uppercase font-bold px-3 py-1 rounded transition-all shadow-sm"
            :class="[
              !canSave(user) 
                ? 'bg-slate-800 text-slate-500 border border-slate-700/60 cursor-not-allowed' 
                : 'bg-emerald-600 hover:bg-emerald-500 text-white cursor-pointer'
            ]"
          >
            Сохранить
          </button>
        </div>
      </div>
    </template>

    <!-- 3. ОБЫЧНЫЙ РЕЖИМ ПРОСМОТРА ПРОФИЛЯ -->
    <template v-else>
      <div
        class="relative z-20 flex items-center justify-between gap-3 transition-transform duration-300 ease-out"
        :class="draggedItem && activeHoverUser === user.id ? 'translate-y-4 translate-x-27' : 'translate-y-0 translate-x-0'"
      >
        <div class="flex items-center gap-2.5 min-w-0">
          <!-- Аватар -->
          <div
            @mousedown.prevent.stop
            @contextmenu.prevent.stop
            @click.prevent.stop="emit('paste-avatar', user, $event)"
            class="relative w-9 h-9 rounded-full bg-slate-800 border border-slate-700 overflow-hidden shrink-0 flex items-center justify-center font-bold text-slate-400 text-xs select-none group/avatar cursor-pointer"
            title="Вставить URL аватара из буфера обмена"
          >
            <img v-if="user.avatar_url" :src="user.avatar_url" class="w-full h-full object-cover" />
            <span v-else>{{ user.name.charAt(0).toUpperCase() }}</span>

            <div class="absolute inset-0 bg-slate-950/75 backdrop-blur-[1px] opacity-0 group-hover/avatar:opacity-100 flex items-center justify-center transition-opacity text-slate-300 hover:text-emerald-400">
              <svg class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <path d="M16 4h2a2 2 0 012 2v14a2 2 0 01-2 2H6a2 2 0 01-2-2V6a2 2 0 012-2h2" />
                <rect x="8" y="2" width="8" height="4" rx="1" ry="1" />
              </svg>
            </div>
          </div>

          <!-- Имя с кнопкой вставки из буфера -->
          <div
            class="flex items-center gap-1.5 min-w-0"
            @mouseenter="emit('mouseenter-name', user.id)"
            @mouseleave="emit('mouseleave-name', user.id)"
          >
            <span class="text-sm font-bold text-white truncate cursor-default">{{ user.name }}</span>
            
            <Transition name="slide-left">
              <button
                v-if="activeNamePasteUsers.has(user.id)"
                @mousedown.prevent.stop
                @contextmenu.prevent.stop
                @click.prevent.stop="emit('paste-name', user, $event)"
                class="text-slate-400 hover:text-emerald-400 text-[11px] transition-colors cursor-pointer p-0.5 shrink-0"
                title="Вставить имя из буфера обмена"
              >
                <svg class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                  <path d="M16 4h2a2 2 0 012 2v14a2 2 0 01-2 2H6a2 2 0 01-2-2V6a2 2 0 012-2h2" />
                  <rect x="8" y="2" width="8" height="4" rx="1" ry="1" />
                </svg>
              </button>
            </Transition>
          </div>
        </div>

        <!-- ТУЛБАР ВЕРХНЕГО УГЛА -->
        <div class="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity" @mousedown.stop>
          <button
            @click="emit('edit-user', user)"
            class="text-slate-500 hover:text-emerald-400 transition-colors p-1 cursor-pointer"
            title="Редактировать лицо"
          >
            <svg class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <path d="M11 4H4a2 2 0 00-2 2v14a2 2 0 002 2h14a2 2 0 002-2v-7" />
              <path d="M18.5 2.5a2.121 2.121 0 013 3L12 15l-4 1 1-4 9.5-9.5z" />
            </svg>
          </button>

          <button
            @click="emit('delete-user', user.id)"
            class="text-slate-500 hover:text-rose-400 transition-colors p-1 cursor-pointer"
            title="Удалить лицо"
          >
            <svg class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <path d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
            </svg>
          </button>
        </div>
      </div>
    </template>

    <!-- 4. ТЕГИ АЛИАСОВ И СЧЕТОВ -->
    <div class="relative z-0 mt-4 space-y-2 cursor-default" @mousedown.stop>
      <!-- Алиасы -->
      <div v-if="(user.aliases || []).length > 0" class="space-y-1">
        <div class="text-[8px] text-slate-500 uppercase font-bold tracking-wider">Алиасы</div>
        <div class="flex flex-wrap gap-1">
          <span
            v-for="alias in user.aliases"
            :key="alias"
            class="text-[9px] px-1.5 py-0.5 rounded border font-mono flex items-center gap-1 transition-colors"
            :class="isSignatureInCache(alias) 
              ? 'bg-slate-800 text-emerald-400 border-slate-700' 
              : 'bg-amber-500/10 text-amber-400 border-amber-500/30'"
            :title="isSignatureInCache(alias) ? 'Подтвержден в кэше' : 'Привязан в core.db, отсутствует в текущем кэше'"
          >
            <span>{{ alias }}</span>
            <button @click.stop="emit('unlink-alias', user, alias)" class="text-slate-500 hover:text-rose-400 font-bold px-0.5 cursor-pointer">✕</button>
          </span>
        </div>
      </div>

      <!-- Счета -->
      <div v-if="(user.ibans || []).length > 0" class="space-y-1">
        <div class="text-[8px] text-slate-500 uppercase font-bold tracking-wider">Счета</div>
        <div class="flex flex-wrap gap-1">
          <span
            v-for="iban in user.ibans"
            :key="iban"
            @click="emit('click-iban', iban)"
            class="text-[9px] px-1.5 py-0.5 rounded border font-mono flex items-center gap-1 cursor-pointer select-none transition-all duration-300 max-w-[260px]"
            :class="isSignatureInCache(iban) 
              ? 'bg-slate-950 text-slate-300 border-slate-800 hover:border-slate-700' 
              : 'bg-amber-500/10 text-amber-400 border-amber-500/30'"
            :title="expandedIbans.has(iban) ? 'Скопировано! Нажмите, чтобы скрыть' : 'Нажмите, чтобы развернуть и скопировать'"
          >
            <span class="inline-block transition-all duration-300 truncate">
              💳 {{ expandedIbans.has(iban) ? iban : formatMaskedIBAN(iban) }}
            </span>
            
            <span v-if="copiedIban === iban" class="text-emerald-400 font-bold text-[9px] animate-fade-in shrink-0">
              ✓
            </span>
          </span>
        </div>
      </div>

      <div v-if="(!user.aliases || user.aliases.length === 0) && (!user.ibans || user.ibans.length === 0)" class="text-[10px] text-slate-600 italic">
        Перетащите алиас...
      </div>
    </div>
  </div>
</template>