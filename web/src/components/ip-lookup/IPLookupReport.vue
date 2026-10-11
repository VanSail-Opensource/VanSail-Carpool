<script setup lang="ts">
import { computed } from 'vue'
import type { IPLookupCheck, IPLookupReport } from '@/api/ipLookup'
import { formatMoney } from '@/utils/format'
import IPLocationMap from './IPLocationMap.vue'
import IPRefusals from './IPRefusals.vue'
import IPRPKIStatus from './IPRPKIStatus.vue'
import IPProxyEvidence from './IPProxyEvidence.vue'
import IPPortProbes from './IPPortProbes.vue'
import IPSubnetGrid from './IPSubnetGrid.vue'
import { providerName } from './presentation'
import { reportRows } from './reportPresentation'
import { ipLocationMap } from './map'

const props = defineProps<{ report: IPLookupReport; requestedIp: string; cacheMatch: 'none' | 'exact' | 'subnet'; check?: IPLookupCheck | null }>()
const rows = computed(() => reportRows(props.report))
const hasMap = computed(() => !!ipLocationMap(props.report.facts.latitude, props.report.facts.longitude))
const databaseIcon = (status: string) => status === 'success' ? 'i-ph-check-circle' : status === 'error' || status === 'partial' ? 'i-ph-warning-circle' : 'i-ph-minus-circle'
</script>

<template>
  <section class="lookup-report" aria-live="polite">
    <header class="report-heading">
      <div><code>{{ requestedIp }}</code><h2 :class="`verdict-${report.verdict}`"><UIcon :name="report.verdict === 'suitable' ? 'i-ph-check-circle' : report.verdict === 'unsuitable' ? 'i-ph-x-circle' : 'i-ph-question'" aria-hidden="true" />{{ $t(`ipLookup.verdicts.${report.verdict}`) }}</h2></div>
      <span>{{ cacheMatch !== 'none' ? $t('ipLookup.cached') : $t('ipLookup.fresh') }}</span>
    </header>
    <p v-if="cacheMatch === 'subnet'" class="report-note">{{ $t('ipLookup.subnetCache', { ip: report.ip }) }}</p>
    <IPRefusals :refusals="report.refusals" />
    <IPRPKIStatus :result="report.rpki" />
    <IPProxyEvidence v-if="report.proxy" :result="report.proxy" />
    <IPPortProbes v-if="report.ports?.length" :ports="report.ports" :source-ip="report.ip" />
    <IPSubnetGrid v-if="report.subnet" :result="report.subnet" :requested-ip="requestedIp" />
    <div class="report-content" :class="{ 'report-content-mapped': hasMap }">
      <dl v-if="rows.length" class="report-facts">
        <div v-for="row in rows" :key="row.key"><dt><UIcon :name="row.icon" aria-hidden="true" />{{ row.label }}</dt><dd>{{ row.value }}<small v-if="row.source">{{ row.source }}</small></dd></div>
      </dl>
      <IPLocationMap :latitude="report.facts.latitude" :longitude="report.facts.longitude" />
    </div>
    <div class="report-databases" :aria-label="$t('ipLookup.providerHeading')">
      <span v-for="database in report.databases" :key="database.id" :title="$t(`ipLookup.statuses.${database.status}`)" :class="`database-${database.status}`"><UIcon :name="databaseIcon(database.status)" aria-hidden="true" />{{ providerName(database.id) }}<span class="sr-only">{{ $t(`ipLookup.statuses.${database.status}`) }}</span></span>
    </div>
    <footer class="report-note">
      <time :datetime="report.checkedAt">{{ $t('ipLookup.checkedAt', { date: new Date(report.checkedAt).toLocaleString() }) }}</time>
      <span v-if="check">{{ check.refunded ? $t('ipLookup.outageRefund') : check.usedQuota ? $t('ipLookup.usedQuotaCount', { count: check.quotaConsumed ?? 1 }) : formatMoney(check.charge) }}</span>
      <span v-else-if="cacheMatch !== 'none'">{{ $t('ipLookup.cachedFree') }}</span>
    </footer>
  </section>
</template>

<style scoped>
.lookup-report { display: grid; gap: 0.65rem; padding: 0.9rem 0; }
.report-heading { display: flex; flex-wrap: wrap; align-items: start; justify-content: space-between; gap: 0.6rem; }
.report-heading span, .report-note, .report-facts dt, .report-facts small { color: var(--text-muted); font-size: 0.82rem; }
.report-heading code { font-size: 1rem; overflow-wrap: anywhere; }
.report-heading > div { display: flex; flex-wrap: wrap; align-items: baseline; gap: 0.35rem 0.75rem; }
.report-heading h2 { display: flex; align-items: center; gap: 0.3rem; margin: 0; font-size: 0.9rem; }
.verdict-unsuitable { color: var(--ui-error); }
.verdict-suitable { color: var(--ui-success); }
.report-note { display: flex; flex-wrap: wrap; gap: 0.25rem 1rem; margin: 0; }
.report-content { display: grid; gap: 1rem; min-width: 0; }
.report-facts { display: grid; align-content: start; margin: 0; }
.report-facts > div { display: grid; grid-template-columns: minmax(0, 0.85fr) minmax(0, 1.15fr); gap: 0.5rem; padding: 0.35rem 0; border-bottom: 1px solid var(--line); }
.report-facts dt { display: flex; align-items: center; gap: 0.3rem; }
.report-facts dd { margin: 0.2rem 0 0; overflow-wrap: anywhere; font-size: 0.9rem; }
.report-facts small { display: block; margin-top: 0.1rem; font-size: 0.7rem; }
.report-databases { display: flex; flex-wrap: wrap; gap: 0.4rem 0.9rem; padding-top: 0.8rem; border-top: 1px solid var(--line); }
.report-databases > span { display: inline-flex; align-items: center; gap: 0.3rem; color: var(--text-muted); font-size: 0.75rem; }
.report-databases .database-error, .report-databases .database-partial { color: var(--warning); }
@media (min-width: 900px) {
  .report-content-mapped { grid-template-columns: minmax(0, 1fr) minmax(0, 1fr); }
  .report-facts { grid-template-columns: repeat(2, minmax(0, 1fr)); column-gap: 1rem; }
  .report-content-mapped .report-facts { grid-template-columns: minmax(0, 1fr); }
}
</style>
