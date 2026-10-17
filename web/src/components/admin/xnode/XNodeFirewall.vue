<script setup lang="ts">
import type { TableColumn } from '@nuxt/ui'
import { computed, onMounted, shallowRef } from 'vue'
import { firewallApi, type FirewallBan } from '@/api/firewall'
import { localizedError, useI18n } from '@/i18n'
const { t } = useI18n()
const items = shallowRef<FirewallBan[]>([])
const loading = shallowRef(true)
const error = shallowRef<string | null>(null)
async function load(): Promise<void> {
  loading.value = true; error.value = null
  try { items.value = (await firewallApi.bans()).items }
  catch (caught) { error.value = localizedError(caught, 'xnode.loadFailed') }
  finally { loading.value = false }
}
const columns = computed<TableColumn<FirewallBan>[]>(() => [
  { accessorKey: 'prefix', header: t('firewall.ip') }, { accessorKey: 'rule', header: t('firewall.rule') },
  { accessorKey: 'count', header: t('firewall.hits') }, { accessorKey: 'detectedAt', header: t('firewall.detectedAt') },
  { accessorKey: 'until', header: t('firewall.deadline') }, { accessorKey: 'pendingNodes', header: t('firewall.pendingNodes') },
])
onMounted(() => void load())
</script>

<template>
  <section class="grid min-w-0 gap-4">
    <div class="flex items-center justify-between gap-4"><p class="text-sm text-muted">{{ t('firewall.reportHint') }}</p><UButton icon="i-ph-arrow-clockwise" variant="ghost" color="neutral" :loading="loading" :aria-label="t('common.refresh')" @click="load" /></div>
    <UProgress v-if="loading" />
    <UAlert v-if="error" color="warning" :title="error" />
    <p v-else-if="!loading && !items.length" class="text-muted">{{ t('firewall.empty') }}</p>
    <div v-else-if="items.length" class="overflow-x-auto">
      <UTable :data="items" :columns="columns" class="admin-table">
        <template #rule-cell="{ row }">{{ t(`firewall.rules.${row.original.rule}`) }}</template>
        <template #detectedAt-cell="{ row }">{{ new Date(row.original.detectedAt).toLocaleString() }}</template>
        <template #until-cell="{ row }">{{ new Date(row.original.until).toLocaleString() }}</template>
      </UTable>
    </div>
  </section>
</template>
