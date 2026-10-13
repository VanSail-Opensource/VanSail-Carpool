<script setup lang="ts">
import { computed, watch } from 'vue'
import type { ConnectivitySnapshot } from '@/api/connectivity'
import InlineNotice from '@/components/common/InlineNotice.vue'
import { useI18n } from '@/i18n'
import ConnectivityAttemptSummary from './ConnectivityAttemptSummary.vue'
import { useConnectivityHistory } from './useConnectivityHistory'

const props = defineProps<{ snapshot: ConnectivitySnapshot | null }>()
const { hostUuid, items, nextCursor, loading, error, load } = useConnectivityHistory()
const { t } = useI18n()
const allHosts = '__all__'
const hostFilter = computed({ get: () => hostUuid.value || allHosts, set: (value: string) => { hostUuid.value = value === allHosts ? '' : value } })
const hostOptions = computed(() => [{ value: allHosts, label: t('hostConnectivity.allHosts') }, ...(props.snapshot?.hosts ?? []).map(host => ({ value: host.hostUuid, label: host.remark || host.address || host.hostUuid }))])
const hosts = computed(() => new Map((props.snapshot?.hosts ?? []).map(host => [host.hostUuid, host.remark || host.address || host.hostUuid])))
watch(() => props.snapshot?.run?.finishedAt, (next, previous) => { if (next && next !== previous) void load() })
</script>

<template>
  <section class="connectivity-history" aria-labelledby="connectivity-history-title">
    <div class="connectivity-history__heading">
      <div><h4 id="connectivity-history-title">{{ t('hostConnectivity.history') }}</h4><p>{{ t('hostConnectivity.historyHint') }}</p></div>
      <UButton color="neutral" variant="ghost" icon="i-ph-arrow-clockwise" :label="t('hostConnectivity.refresh')" :loading="loading" :disabled="loading" @click="load()" />
    </div>
    <UFormField name="connectivity-history-host" :label="t('hostConnectivity.hostFilter')">
      <USelect id="connectivity-history-host" v-model="hostFilter" class="w-full" :items="hostOptions" value-key="value" />
    </UFormField>
    <InlineNotice v-if="error" tone="warning">{{ error }}</InlineNotice>
    <UTable class="admin-table" :data="items" :loading="loading" :columns="[{ id: 'host', header: t('adminWorkspace.name') }, { id: 'trigger', header: t('adminWorkspace.type') }, { id: 'result', header: t('adminWorkspace.results') }]">
      <template #host-cell="{ row }"><strong>{{ row.original.hostUuid ? hosts.get(row.original.hostUuid) || row.original.hostUuid : t('hostConnectivity.setupAttempt') }}</strong></template>
      <template #trigger-cell="{ row }">{{ t(`hostConnectivity.trigger.${row.original.trigger}`) }}</template>
      <template #result-cell="{ row }"><ConnectivityAttemptSummary :attempt="row.original" /></template>
      <template #empty>{{ t('hostConnectivity.noHistory') }}</template>
    </UTable>
    <UButton v-if="nextCursor" color="neutral" variant="outline" :label="t('hostConnectivity.loadMore')" :loading="loading" :disabled="loading" @click="load(true)" />
  </section>
</template>

<style scoped>
.connectivity-history { display: grid; gap: 0.7rem; padding-top: 0.8rem; border-top: 1px solid var(--line); }
.connectivity-history__heading { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 0.5rem; }
.connectivity-history__heading h4 { margin: 0; font-size: 0.85rem; }
.connectivity-history__heading p { margin: 0.2rem 0 0; color: var(--text-muted); font-size: 0.75rem; }
.connectivity-history__list { list-style: none; margin: 0; padding: 0; }
.connectivity-history__row { display: grid; gap: 0.55rem; padding: 0.75rem 0; border-bottom: 1px solid var(--line); }
.connectivity-history__row:last-child { border-bottom: 0; }
.connectivity-history__identity { display: grid; gap: 0.2rem; min-width: 0; overflow-wrap: anywhere; }
.connectivity-history__identity strong { font-size: 0.85rem; font-weight: 600; }
.connectivity-history__identity small, .connectivity-history__empty { color: var(--text-muted); font-size: 0.75rem; }
.connectivity-history__empty { margin: 0; }
@media (min-width: 640px) { .connectivity-history__row { grid-template-columns: minmax(0, 1fr) minmax(0, 1fr); } }
</style>
