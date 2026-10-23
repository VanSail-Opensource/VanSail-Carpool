<script setup lang="ts">
import { computed, reactive, shallowRef } from 'vue'

import type { Combo, SquadProduct } from '@/api/adminOperations'
import { useI18n } from '@/i18n'

export interface AdminUserSearchFiltersValue {
  state: '' | 'active' | 'non_active'
  comboIds: string[]
  squadUuids: string[]
  match: 'and' | 'or'
}

type SelectedState = 'all' | Exclude<AdminUserSearchFiltersValue['state'], ''>

const props = defineProps<{ combos: readonly Combo[]; squads: readonly SquadProduct[] }>()
const emit = defineEmits<{ apply: [value: AdminUserSearchFiltersValue] }>()
const { t } = useI18n()
const drawerOpen = shallowRef(false)
const value = reactive<{ state: SelectedState; comboIds: string[]; squadUuids: string[]; match: 'and' | 'or' }>({ state: 'all', comboIds: [], squadUuids: [], match: 'and' })
const stateItems = computed(() => [
  { label: t('adminUsers.allStates'), value: 'all' },
  { label: t('adminUsers.activeEntitlement'), value: 'active' },
  { label: t('adminUsers.nonActiveEntitlement'), value: 'non_active' },
])
const matchItems = computed(() => [
  { label: t('adminUsers.matchAnd'), value: 'and' },
  { label: t('adminUsers.matchOr'), value: 'or' },
])
const comboItems = computed(() => props.combos.map((item) => ({ label: item.name, value: item.id })))
const squadItems = computed(() => props.squads.map((item) => ({ label: item.name, value: item.groupId })))

function apply(): void {
  emit('apply', { state: value.state === 'all' ? '' : value.state, comboIds: [...value.comboIds], squadUuids: [...value.squadUuids], match: value.match })
  drawerOpen.value = false
}

function clear(): void {
  value.state = 'all'
  value.comboIds = []
  value.squadUuids = []
  value.match = 'and'
  apply()
}
</script>

<template>
  <UPopover :open="drawerOpen" @update:open="drawerOpen = $event">
    <UButton color="neutral" variant="outline" icon="i-ph-funnel" :label="t('adminUsers.filters')" @click="drawerOpen = true" />
    <template #content>
      <div class="admin-user-filter-fields">
        <UFormField :label="t('adminUsers.entitlementState')"><USelect v-model="value.state" :items="stateItems" value-key="value" /></UFormField>
        <UFormField :label="t('adminUsers.combos')"><USelectMenu v-model="value.comboIds" :items="comboItems" value-key="value" multiple /></UFormField>
        <UFormField :label="t('adminUsers.squads')"><USelectMenu v-model="value.squadUuids" :items="squadItems" value-key="value" multiple /></UFormField>
        <UFormField :label="t('adminUsers.match')"><URadioGroup v-model="value.match" :items="matchItems" value-key="value" orientation="horizontal" /></UFormField>
        <div class="button-row"><UButton color="neutral" variant="ghost" :label="t('common.clear')" @click="clear" /><UButton :label="t('adminUsers.applyFilters')" @click="apply" /></div>
      </div>
    </template>
  </UPopover>
</template>
<style scoped>
.admin-user-filter-fields { display: grid; gap: .85rem; width: min(350px, calc(100vw - 2rem)); padding: 1rem; }
.admin-user-filter-fields :deep([data-slot="root"]) { width: 100%; }
</style>
