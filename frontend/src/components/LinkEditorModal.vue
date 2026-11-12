<script setup>
import { ref, computed, watch } from 'vue'
import { formatMaskedIBAN, validateAndCleanAccount, isMaskedPan } from '../utils/formatters.js'
import { isIbanLike, idKey, detectBankKey } from '../utils/personLayout.js'
import { prepareAccount } from '../utils/accountView.js'
import { bankTheme } from '../utils/bankTheme.js'
import AccountPanel from './AccountPanel.vue'
import { readClipboardText } from '../utils/clipboard.js'
import UiModal from './ui/UiModal.vue'
import UiIcon from './ui/UiIcon.vue'
import UiIconButton from './ui/UiIconButton.vue'
import UiButton from './ui/UiButton.vue'
import UiDot from './ui/UiDot.vue'
import IcuBondsEditor from './IcuBondsEditor.vue'

const props = defineProps({
  isOpen: { type: Boolean, default: false },
  aliasData: { type: Object, default: null },
  users: { type: Array, default: () => [] },
  accounts: { type: Array, default: () => [] },
  isSignatureInCache: { type: Function, required: true }
})

const emit = defineEmits(['close', 'save-link', 'quick-create-user'])

const selectedUserId = ref('')
const initialUserId = ref('')

// Дерево: { [IBAN]: [PAN, ...], '': [PAN без IBAN] }
const accountTree = ref({})
const initialTreeJson = ref('')

const rootAccountInput = ref('')
const activeParentIban = ref(null)
const childAccountInput = ref('')
const activeBondsIban = ref(null) // у счёта ICU вместо карт — форма облигаций
const isIcuIban = (iban) => !!iban && detectBankKey(iban, null, []) === 'icu'

const LOOSE = ''

// IBAN-ы по порядку, «карты без IBAN» — последними
const treeEntries = computed(() => {
  const keys = Object.keys(accountTree.value)
  const ordered = keys.filter(k => k !== LOOSE)
  if (keys.includes(LOOSE)) ordered.push(LOOSE)
  return ordered.map(k => [k, accountTree.value[k] || []])
})

// Дерево строится только из эффективной структуры алиаса (статика ∪ кэш − баны),
// родитель карты приходит уже разрешённым — никаких догадок «по первому IBAN».
const buildTree = (accounts) => {
  const items = []
  const seen = new Set()
  ;(Array.isArray(accounts) ? accounts : []).forEach(a => {
    const id = String(a?.account_identifier || '').trim()
    const k = idKey(id)
    if (!id || seen.has(k)) return
    seen.add(k)
    items.push({ id, key: k, parent: String(a.parent_identifier || '').trim() })
  })

  const tree = {}
  const ibanById = new Map()
  items.filter(it => isIbanLike(it.id)).forEach(it => {
    tree[it.id] = []
    ibanById.set(it.key, it.id)
  })
  items.filter(it => !isIbanLike(it.id)).forEach(it => {
    const root = ibanById.get(idKey(it.parent)) ?? LOOSE
    ;(tree[root] ||= []).push(it.id)
  })
  return tree
}

const flattenTree = (tree) => {
  const out = []
  Object.keys(tree).forEach(root => {
    if (root !== LOOSE) out.push({ account_identifier: root, parent_identifier: '' })
    ;(tree[root] || []).forEach(pan => out.push({ account_identifier: pan, parent_identifier: root }))
  })
  return out
}

watch(
  [() => props.isOpen, () => props.aliasData],
  ([isOpen, aliasData]) => {
    if (!isOpen || !aliasData) {
      accountTree.value = {}
      initialTreeJson.value = ''
      return
    }

    selectedUserId.value = aliasData.user_id || ''
    initialUserId.value = selectedUserId.value

    const tree = buildTree(aliasData.accounts)
    accountTree.value = tree
    initialTreeJson.value = JSON.stringify(tree)

    rootAccountInput.value = ''
    activeParentIban.value = null
    childAccountInput.value = ''
    activeBondsIban.value = null
  },
  { immediate: true }
)

const flatIdsList = computed(() => flattenTree(accountTree.value).map(a => a.account_identifier))
const hasId = (id) => flatIdsList.value.some(x => idKey(x) === idKey(id))

// Настоящий номер поверх уже известной маски (те же начало и конец): карта остаётся на месте
// и главной, но в списке (и потом в базе) теперь чистое число. Возвращает true, если это был апгрейд.
const upgradeMasked = (clean) => {
  if (isMaskedPan(clean)) return false
  const k = idKey(clean)
  for (const root of Object.keys(accountTree.value)) {
    const list = accountTree.value[root] || []
    const i = list.findIndex(x => idKey(x) === k && isMaskedPan(x))
    if (i !== -1) {
      const next = [...list]
      next[i] = clean
      accountTree.value = { ...accountTree.value, [root]: next }
      return true
    }
  }
  return false
}

const structuredAccountsPayload = computed(() => flattenTree(accountTree.value))

const hasChanges = computed(() => {
  if (selectedUserId.value !== initialUserId.value) return true
  return JSON.stringify(accountTree.value) !== initialTreeJson.value
})

const isRootInputValid = computed(() => {
  if (!rootAccountInput.value) return false
  return !!validateAndCleanAccount(rootAccountInput.value)
})

const isChildInputValid = computed(() => {
  if (!childAccountInput.value) return false
  const clean = validateAndCleanAccount(childAccountInput.value)
  return !!clean && !isIbanLike(clean)
})

// IBAN → новый корень; номер карты без IBAN → в группу «карты без IBAN»
const addRootAccount = () => {
  const clean = validateAndCleanAccount(rootAccountInput.value)
  if (!clean) return
  if (upgradeMasked(clean)) { rootAccountInput.value = ''; return }
  if (hasId(clean)) return
  const tree = { ...accountTree.value }
  if (isIbanLike(clean)) {
    tree[clean] = []
  } else {
    tree[LOOSE] = [...(tree[LOOSE] || []), clean]
  }
  accountTree.value = tree
  rootAccountInput.value = ''
}

const addChildAccount = (parentIban) => {
  const clean = validateAndCleanAccount(childAccountInput.value)
  if (!clean || isIbanLike(clean) || !accountTree.value[parentIban]) return
  if (upgradeMasked(clean)) { childAccountInput.value = ''; activeParentIban.value = null; return }
  if (hasId(clean)) return
  accountTree.value = { ...accountTree.value, [parentIban]: [...accountTree.value[parentIban], clean] }
  childAccountInput.value = ''
  activeParentIban.value = null
}

const toggleAddChildInput = (parentIban) => {
  if (activeParentIban.value === parentIban) {
    activeParentIban.value = null
  } else {
    activeParentIban.value = parentIban
    childAccountInput.value = ''
  }
}

const removeAccount = (item, parentIban = null) => {
  const newTree = { ...accountTree.value }
  if (parentIban !== null) {
    newTree[parentIban] = (newTree[parentIban] || []).filter(i => i !== item)
    if (parentIban === LOOSE && newTree[LOOSE].length === 0) delete newTree[LOOSE]
  } else {
    delete newTree[item] // карты этого IBAN уходят вместе с ним
  }
  accountTree.value = newTree
}

const pasteFromClipboard = async (isChild = false) => {
  try {
    const text = await readClipboardText()
    const clean = validateAndCleanAccount(text)
    if (!clean) return
    if (isChild && activeParentIban.value) {
      childAccountInput.value = clean
      addChildAccount(activeParentIban.value)
    } else {
      rootAccountInput.value = clean
      addRootAccount()
    }
  } catch (err) {
    console.error('Paste error:', err)
  }
}

// Цветная карта вместо текстовой строки: счёт из кэша — как в плитках Лиц (банк, баланс), иначе плитка по IBAN/BIN.
// Список карт на плитке — именно карты дерева (в том же порядке), чтобы × убирал нужную.
const cardOf = (iban, pans) => {
  const key = idKey(iban)
  const acc = iban ? props.accounts.find(a => idKey(a.iban || a.account_identifier) === key) : null
  const base = acc
    ? prepareAccount(acc)
    : { displayIban: iban ? formatMaskedIBAN(iban) : 'КАРТА', balance: null, theme: bankTheme(detectBankKey(iban || '', null, pans), '') }
  return { ...base, manual: false, pans: pans.map(formatMaskedIBAN) } // карандаш журнала только в Операционной
}
const looseCard = (pan) => ({ displayIban: 'КАРТА', balance: null, pans: [formatMaskedIBAN(pan)], theme: bankTheme(detectBankKey('', null, [pan]), '') })

// Цвет строки счёта: зелёный — подтверждён кэшем, жёлтый — только в памяти
const rowTone = (id) =>
  props.isSignatureInCache(id) ? 'border-emerald-500/40 text-emerald-300' : 'border-amber-500/40 text-amber-300'

const handleQuickCreate = () => {
  const defaultName = props.aliasData?.name || 'Новое лицо'
  emit('quick-create-user', defaultName, (newCreatedId) => {
    if (newCreatedId) {
      selectedUserId.value = newCreatedId
    }
  })
}

const handleSave = () => {
  if (!hasChanges.value) return

  let initialTreeObj = {}
  try {
    initialTreeObj = JSON.parse(initialTreeJson.value)
  } catch (e) {}

  emit('save-link', {
    alias: props.aliasData?.name,
    user_id: selectedUserId.value || '',
    accounts: structuredAccountsPayload.value,
    initialAccounts: flattenTree(initialTreeObj)
  })
}
</script>

<template>
  <UiModal :is-open="isOpen" max-width="max-w-lg" @close="emit('close')">
    <!-- Шапка -->
    <div class="flex items-center justify-between">
      <div class="flex items-center gap-3">
        <div class="flex items-center gap-2">
          <UiIcon name="edit" size="lg" class="text-emerald-400" />
          <span class="text-sm font-bold text-white uppercase tracking-wider">АЛИАС</span>
        </div>

        <UiButton variant="primary" size="md" class="tracking-wider" :disabled="!hasChanges" @click="handleSave">
          Сохранить
        </UiButton>
      </div>

      <UiIconButton icon="close" title="Закрыть" @click="emit('close')" />
    </div>

    <!-- Имя Алиаса -->
    <div
      class="px-3.5 py-2.5 rounded border flex items-center justify-between font-bold text-sm transition-colors"
      :class="isSignatureInCache(aliasData?.name)
        ? 'bg-emerald-950/40 border-emerald-500/40 text-emerald-400'
        : 'bg-amber-950/40 border-amber-500/40 text-amber-400'"
    >
      <span>{{ aliasData?.name || '—' }}</span>
    </div>

    <!-- Выбор привязанного лица -->
    <div class="space-y-1.5">
      <div class="ui-label">ЛИЦО</div>
      <div class="flex items-center gap-2">
        <select v-model="selectedUserId" class="ui-select !py-2">
          <option value="" class="bg-slate-950 text-slate-400 py-1">Нет</option>
          <option v-for="u in users" :key="u.id" :value="u.id" class="bg-slate-950 text-slate-200 py-1">
            {{ u.name }}
          </option>
        </select>

        <UiButton variant="primary" class="!py-2 shrink-0" title="Создать новое Лицо" @click="handleQuickCreate">
          Создать
        </UiButton>
      </div>
    </div>

    <!-- Список счетов -->
    <div class="space-y-2">
      <div class="flex items-center justify-between ui-label">
        <span>СЧЕТА</span>
        <span class="normal-case font-normal flex items-center gap-2">
          <span class="flex items-center gap-1"><UiDot cached />в кэше</span>
          <span class="flex items-center gap-1"><UiDot />только в памяти</span>
        </span>
      </div>

      <div class="space-y-2 max-h-80 overflow-y-auto pr-1">
        <template v-for="[parentIban, children] in treeEntries" :key="parentIban || '__loose__'">
          <!-- IBAN: цветная карта + управление -->
          <div v-if="parentIban" class="relative">
            <AccountPanel
              :card="cardOf(parentIban, children)"
              :cached="isSignatureInCache(parentIban)"
              compact
              removable-pans
              @remove-pan="removeAccount(children[$event], parentIban)"
            />
            <div class="absolute right-2 bottom-2 flex items-center gap-1.5 z-20">
              <UiButton
                v-if="isIcuIban(parentIban)"
                :variant="activeBondsIban === parentIban ? 'cancel' : 'outline'"
                size="sm"
                title="Облигации счёта ICU"
                @click="activeBondsIban = activeBondsIban === parentIban ? null : parentIban"
              >
                {{ activeBondsIban === parentIban ? 'Скрыть' : 'Облигации' }}
              </UiButton>
              <UiButton
                v-else
                :variant="activeParentIban === parentIban ? 'cancel' : 'outline'"
                size="sm"
                :title="activeParentIban === parentIban ? 'Отменить добавление' : 'Привязать карту к этому IBAN'"
                @click="toggleAddChildInput(parentIban)"
              >
                {{ activeParentIban === parentIban ? 'Отмена' : 'Добавить' }}
              </UiButton>
              <UiIconButton icon="close" tone="danger" title="Удалить счет" @click="removeAccount(parentIban)" />
            </div>
          </div>

          <div v-else class="px-3 py-1 ui-label">Карты без IBAN</div>

          <!-- Счёт ICU: облигации вместо карт -->
          <IcuBondsEditor v-if="parentIban && activeBondsIban === parentIban" :iban="parentIban" />

          <!-- Ввод карты (PAN) под IBAN -->
          <div v-if="activeParentIban === parentIban" class="ml-5 flex items-center gap-2 pt-0.5 pb-1">
            <span class="text-slate-500 font-bold select-none text-xs">└─</span>
            <div class="relative flex-1 flex items-center">
              <input
                v-model="childAccountInput"
                type="text"
                placeholder="Номер карты, можно 414951******5262"
                class="ui-input pr-7"
                :class="childAccountInput && !isChildInputValid && 'ui-input--invalid'"
                @keyup.enter="addChildAccount(parentIban)"
              />
              <UiIconButton icon="paste" tone="accent" size="sm" class="absolute right-1" @click="pasteFromClipboard(true)" />
            </div>
            <UiButton variant="primary" size="sm" :disabled="!isChildInputValid" @click="addChildAccount(parentIban)">ОК</UiButton>
          </div>

          <!-- Карты без IBAN: каждая своей плиткой -->
          <template v-if="!parentIban">
            <AccountPanel
              v-for="child in children"
              :key="child"
              :card="looseCard(child)"
              :cached="isSignatureInCache(child)"
              compact
              removable-pans
              @remove-pan="removeAccount(child, parentIban)"
            />
          </template>
        </template>

        <div
          v-if="treeEntries.length === 0"
          class="text-slate-600 italic text-[11px] p-2 text-center border border-dashed border-slate-800 rounded"
        >
          Счетов пока не привязано
        </div>
      </div>

      <!-- Добавление IBAN или карты без IBAN -->
      <div class="flex items-center gap-2 pt-1">
        <div class="relative flex-1 flex items-center">
          <input
            v-model="rootAccountInput"
            type="text"
            placeholder="IBAN или номер карты (можно со звёздочками)"
            class="ui-input pr-8"
            :class="rootAccountInput && !isRootInputValid && 'ui-input--invalid'"
            @keyup.enter="addRootAccount"
          />
          <UiIconButton icon="paste" tone="accent" class="absolute right-1" title="Вставить из буфера обмена" @click="pasteFromClipboard(false)" />
        </div>

        <UiButton variant="primary" class="shrink-0" :disabled="!isRootInputValid" @click="addRootAccount">Добавить</UiButton>
      </div>
    </div>
  </UiModal>
</template>
