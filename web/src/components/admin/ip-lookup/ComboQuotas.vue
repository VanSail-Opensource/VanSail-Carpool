<script setup lang="ts">
import type { Combo } from '@/api/types'

const quotas = defineModel<Record<string, number | null>>({ required: true })
defineProps<{ combos: Combo[] }>()
</script>

<template>
  <section class="combo-quotas">
    <h2>{{ $t('ipLookup.quotaHeading') }}</h2><p>{{ $t('ipLookup.quotaHelp') }}</p>
    <UTable class="admin-table" :data="combos" :columns="[{ accessorKey: 'name', header: $t('adminWorkspace.name') }, { id: 'quota', header: $t('ipLookup.quotaHeading') }]">
      <template #quota-cell="{ row }"><UFormField :name="`quota-${row.original.id}`"><UInputNumber v-model="quotas[row.original.id]" :aria-label="row.original.name" :min="0" :max="1000000" :step="1" :placeholder="$t('ipLookup.unconfigured')" /></UFormField></template>
    </UTable>
  </section>
</template>

<style scoped>
.combo-quotas { display: grid; gap: 0.8rem; }
.combo-quotas h2 { font-size: 1rem; margin: 0; }
.combo-quotas p { color: var(--text-muted); font-size: 0.82rem; margin: 0; }
@media (min-width: 900px) { .quota-row { max-width: 28rem; } }
</style>
