<script setup>
import { computed } from 'vue'
import SectionRail from '../components/SectionRail.vue'
import OperationalView from './OperationalView.vue'
import PersonsView from './PersonsView.vue'
import { useSection } from '../composables/useSection.js'
import { DATA_SECTIONS } from '../utils/nav.js'

const props = defineProps({
  accounts: { type: Array, default: () => [] },
  transactions: { type: Array, default: () => [] },
  users: { type: Array, default: () => [] },
  dataVersion: { default: null },
  personsCount: { type: Number, default: 0 }
})
const emit = defineEmits(['refresh'])

const section = useSection('economic_data_section', DATA_SECTIONS, 'history')
const sections = computed(() => [
  { key: 'history', label: 'История', icon: 'history' },
  { key: 'persons', label: 'Лица', icon: 'users', count: props.personsCount }
])
</script>

<template>
  <div class="relative">
    <SectionRail v-model="section" :sections="sections" label="Разделы учета" />
    <KeepAlive><OperationalView v-if="section === 'history'" :accounts="accounts" :transactions="transactions" :users="users" /></KeepAlive>
    <KeepAlive><PersonsView v-if="section === 'persons'" :transactions="transactions" :accounts="accounts" :data-version="dataVersion" @refresh="emit('refresh')" /></KeepAlive>
  </div>
</template>
