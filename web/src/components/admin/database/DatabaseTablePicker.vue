<script setup lang="ts">
import { computed, shallowRef } from 'vue'

import type { DatabaseTable } from '@/api/features'
import { useI18n } from '@/i18n'
import type { DeepReadonly } from './types'

const props = defineProps<{
  tables: DeepReadonly<DatabaseTable[]>
  selected: string | null
  busy: boolean
}>()
const emit = defineEmits<{ select: [name: string] }>()
const { t } = useI18n()
const search = shallowRef('')

const items = computed(() => props.tables.map((table) => ({
  value: table.name,
  label: table.name,
  description: table.highRisk ? t('adminDatabase.highRisk') : undefined,
  icon: 'i-ph-database',
})))
const visibleTables = computed(() => {
  const needle = search.value.trim().toLowerCase()
  return needle ? props.tables.filter((table) => table.name.toLowerCase().includes(needle)) : props.tables
})

function choose(value: unknown): void {
  const name = String(value)
  if (!name || name === props.selected || props.busy) return

  emit('select', name)
}

</script>
<template>
  <nav class="database-table-picker" :aria-label="t('adminDatabase.tables')">
    <div class="database-table-picker__mobile"><UFormField :label="t('adminDatabase.tables')"><USelectMenu class="database-table-picker__select" :model-value="selected ?? undefined" :items="items" value-key="value" label-key="label" :search-input="{ placeholder: t('adminDatabase.tableSearch') }" :placeholder="t('adminDatabase.selectTable')" :disabled="busy" :aria-label="t('adminDatabase.selectTable')" @update:model-value="choose" /></UFormField></div>
    <div class="database-table-picker__desktop"><UInput v-model="search" icon="i-ph-magnifying-glass" :placeholder="t('adminDatabase.tableSearch')" :aria-label="t('adminDatabase.tableSearch')" /><UButton v-for="table in visibleTables" :key="table.name" class="database-table-picker__button" :class="{ 'database-table-picker__button--active': selected === table.name }" color="neutral" variant="ghost" icon="i-ph-database" :disabled="busy" :aria-pressed="selected === table.name" @click="choose(table.name)"><span>{{ table.name }}</span><small v-if="table.highRisk">{{ t('adminDatabase.highRisk') }}</small></UButton></div>
  </nav>
</template>
<style scoped>
.database-table-picker { min-width: 0; }
.database-table-picker__select { width: 100%; }
.database-table-picker__mobile { display: none; }
.database-table-picker__desktop { display: grid; gap: 6px; }
.database-table-picker__button { justify-content: flex-start; border-radius: 6px; }
.database-table-picker__button--active { background: var(--accent-soft); color: var(--accent); }
.database-table-picker__button small { margin-inline-start: auto; font-size: 10px; }
@media (max-width: 900px) { .database-table-picker__mobile { display: block; } .database-table-picker__desktop { display: none; } }
</style>
