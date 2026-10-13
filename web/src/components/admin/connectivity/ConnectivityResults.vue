<script setup lang="ts">
import type { ConnectivitySnapshot } from '@/api/connectivity'
import InlineNotice from '@/components/common/InlineNotice.vue'
import { useI18n } from '@/i18n'
import ConnectivityAttemptSummary from './ConnectivityAttemptSummary.vue'
import { connectivityError } from './presentation'

defineProps<{ snapshot: ConnectivitySnapshot }>()
const { t } = useI18n()
</script>

<template>
  <section class="connectivity-results" aria-labelledby="connectivity-results-title">
    <div class="connectivity-results__heading">
      <h4 id="connectivity-results-title">{{ t('hostConnectivity.latest') }}</h4>
      <span v-if="snapshot.run" role="status">{{ t(`hostConnectivity.run.${snapshot.run.status}`, { completed: snapshot.run.completed, total: snapshot.run.total }) }}</span>
    </div>
    <InlineNotice v-if="snapshot.stale" tone="warning">{{ t('hostConnectivity.stale') }}</InlineNotice>
    <InlineNotice v-if="snapshot.errorCode" tone="warning">{{ connectivityError(snapshot.errorCode) }}</InlineNotice>
    <InlineNotice v-else-if="snapshot.run?.errorCode" tone="warning">{{ connectivityError(snapshot.run.errorCode) }}</InlineNotice>
    <UTable v-if="snapshot.hosts.length || !snapshot.errorCode" class="admin-table" :data="snapshot.hosts" :columns="[{ id: 'host', header: t('adminWorkspace.name') }, { id: 'endpoint', header: t('adminWorkspace.identity') }, { id: 'result', header: t('adminWorkspace.results') }]">
      <template #host-cell="{ row }"><strong>{{ row.original.remark || row.original.address || row.original.hostUuid }}</strong></template>
      <template #endpoint-cell="{ row }"><code v-if="row.original.address && row.original.port > 0">{{ t('hostConnectivity.endpoint', { address: row.original.address, port: row.original.port }) }}</code></template>
      <template #result-cell="{ row }"><ConnectivityAttemptSummary :attempt="row.original.latest" /></template>
      <template #empty>{{ t('hostConnectivity.noHosts') }}</template>
    </UTable>
  </section>
</template>

<style scoped>
.connectivity-results { display: grid; gap: 0.7rem; padding-top: 0.8rem; border-top: 1px solid var(--line); }
.connectivity-results__heading { display: flex; flex-wrap: wrap; align-items: baseline; justify-content: space-between; gap: 0.5rem; }
.connectivity-results__heading h4 { margin: 0; font-size: 0.85rem; }
.connectivity-results__heading span, .connectivity-results__empty { color: var(--text-muted); font-size: 0.8rem; }
.connectivity-results__empty { margin: 0; }
.connectivity-results__list { list-style: none; margin: 0; padding: 0; }
.connectivity-results__row { display: grid; gap: 0.55rem; padding: 0.75rem 0; border-bottom: 1px solid var(--line); }
.connectivity-results__row:last-child { border-bottom: 0; }
.connectivity-results__identity { display: grid; gap: 0.2rem; min-width: 0; overflow-wrap: anywhere; }
.connectivity-results__identity strong { font-size: 0.85rem; font-weight: 600; }
.connectivity-results__identity small { color: var(--text-muted); }
@media (min-width: 640px) { .connectivity-results__row { grid-template-columns: minmax(0, 1fr) minmax(0, 1fr); } }
</style>
