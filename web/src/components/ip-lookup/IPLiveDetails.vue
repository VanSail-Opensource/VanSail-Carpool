<script setup lang="ts">
import type { IPLookupReport, IPLookupLiveDetails, IPLookupQuote } from '@/api/ipLookup'
import { formatMoney } from '@/utils/format'
import { useLiveIPDetails } from './useLiveIPDetails'
import InlineNotice from '@/components/common/InlineNotice.vue'
import SkeletonBlock from '@/components/common/SkeletonBlock.vue'
import IPBGPTopology from './IPBGPTopology.vue'
import IPASNDetails from './IPASNDetails.vue'
import LiveSourceNote from './LiveSourceNote.vue'
import IPLookupProgress from './IPLookupProgress.vue'
import IPTransferHistory from './IPTransferHistory.vue'
const props = defineProps<{ requestedIp: string; report: IPLookupReport; snapshot: IPLookupLiveDetails | null; refreshQuote: IPLookupQuote | null; busy: boolean }>()
defineEmits<{ refresh: [] }>()
const { details, progress, loading, errorCode } = useLiveIPDetails(() => props.requestedIp, () => props.report.status === 'processing' ? '' : `${props.report.id}:${props.report.checkedAt}`, () => props.snapshot)
</script>

<template>
  <section class="live-details">
    <header class="live-heading"><h2><UIcon name="i-ph-network" aria-hidden="true" />{{ $t('ipLookupLive.title') }}</h2><UButton v-if="refreshQuote" color="neutral" variant="ghost" icon="i-ph-arrow-clockwise" :loading="busy" :disabled="loading || busy" :label="refreshQuote.useQuota ? $t('ipLookup.refreshIncluded', { count: refreshQuote.quotaCost ?? 1 }) : $t('ipLookupLive.refresh', { fee: formatMoney(refreshQuote.charge) })" @click="$emit('refresh')" /></header>
    <IPLookupProgress v-if="loading" :progress="progress" />
    <SkeletonBlock v-if="loading && !details" height="8rem" />
    <InlineNotice v-if="errorCode" tone="warning">{{ $t(`ipLookupLive.errors.${errorCode}`) }}</InlineNotice>
    <template v-if="details">
      <p v-if="details.cacheMatch === 'subnet'" class="live-fetch-time">{{ $t('ipLookupLive.cacheSubnet', { ip: details.sourceIP ?? details.ip!, date: new Date(details.fetchedAt!).toLocaleString() }) }}</p>
      <p v-else-if="details.cacheMatch === 'exact'" class="live-fetch-time">{{ $t('ipLookupLive.cacheExact', { date: new Date(details.fetchedAt!).toLocaleString() }) }}</p>
      <IPBGPTopology v-if="details.topology" :topology="details.topology" />
      <IPASNDetails v-if="details.asns" :asns="details.asns" />
      <div class="live-ip-facts">
        <section v-if="details.scores"><h3>{{ $t('ipLookupLive.scores') }}</h3><dl><div v-for="kind in ['company', 'asn'] as const" :key="kind"><dt>{{ $t(`ipLookup.scores.${kind}_abuse_ratio`) }}</dt><dd>{{ details.scores[kind].ratio === null ? $t('ipLookupLive.status.empty') : `${(details.scores[kind].ratio! * 100).toFixed(3)}%` }}<span v-if="details.scores[kind].label">{{ details.scores[kind].label }}</span></dd></div></dl><LiveSourceNote :meta="details.scores" /></section>
        <section v-if="details.block"><h3>{{ $t('ipLookupLive.blockTitle') }}</h3><code v-if="details.block.prefix">{{ details.block.prefix }}</code><p v-if="details.block.errorCode !== 'IP_DETAILS_LEGACY'">{{ $t('ipLookupLive.blockCount', { reported: details.block.reportedAddresses ?? $t('ipLookupLive.status.empty'), capacity: details.block.addressCapacity || $t('ipLookupLive.status.empty'), days: details.block.windowDays }) }}</p><p v-if="details.block.errorCode === 'IP_DETAILS_LEGACY'">{{ $t('ipLookupLive.legacyData') }}</p><LiveSourceNote v-else :meta="details.block" /></section>
        <section v-if="details.ptr" class="ptr-facts"><h3>{{ $t('ipLookupLive.ptrTitle') }}</h3><ul v-if="details.ptr.domains.length"><li v-for="domain in details.ptr.domains" :key="domain"><code>{{ domain }}</code></li></ul><LiveSourceNote :meta="details.ptr" /></section>
      </div>
      <IPTransferHistory :history="details.transfers" :loading="loading && !details.transfers" :failed="!!errorCode" />
      <section v-if="details.shodan" class="live-shodan"><h3>{{ $t('ipLookupLive.shodanTitle') }}</h3><dl><div><dt>{{ $t('ipLookupLive.shodanHostnames') }}</dt><dd>{{ details.shodan.hostnames.length ? details.shodan.hostnames.join(', ') : $t('ipLookupLive.status.empty') }}</dd></div><div><dt>{{ $t('ipLookupLive.shodanPorts') }}</dt><dd><code>{{ details.shodan.ports.length ? details.shodan.ports.join(', ') : $t('ipLookupLive.status.empty') }}</code></dd></div></dl><LiveSourceNote :meta="details.shodan" /><details><summary>{{ $t('ipLookupLive.about') }}</summary><p>{{ $t('ipLookupLive.shodanContext') }}</p></details></section>
      <p v-if="details.fetchedAt" class="live-fetch-time">{{ $t('ipLookupLive.fetchedAt', { date: new Date(details.fetchedAt).toLocaleString() }) }}</p>
    </template>
  </section>
</template>

<style scoped>
.live-details { min-width: 0; padding-top: 0.75rem; border-top: 1px solid var(--line); }
.live-heading { display: flex; align-items: center; justify-content: space-between; gap: 0.8rem; margin-bottom: 0.4rem; }
.live-heading h2 { display: inline-flex; gap: 0.4rem; align-items: center; }
.live-heading h2, .live-ip-facts h3 { font-size: 1rem; margin: 0; }
.live-heading p, .live-fetch-time { font-size: 0.78rem; color: var(--text-muted); margin: 0.35rem 0 0; line-height: 1.5; }
.live-ip-facts { display: grid; gap: 0.8rem; padding-block: 0.75rem; border-top: 1px solid var(--line); }
.live-ip-facts section { min-width: 0; }
.live-ip-facts dl { margin: 0.6rem 0 0; display: grid; gap: 0.6rem; }
.live-ip-facts dt { color: var(--text-muted); font-size: 0.78rem; }
.live-ip-facts dd { margin: 0.25rem 0 0; font-size: 0.88rem; }
.live-ip-facts dd span { margin-left: 0.5rem; color: var(--text-muted); }
.live-ip-facts p, .live-ip-facts code { font-size: 0.82rem; overflow-wrap: anywhere; }
.live-ip-facts .ptr-facts ul { margin: 0.3rem 0; padding: 0; list-style: none; display: grid; gap: 0.2rem; }
.live-shodan { padding-block: 1rem; border-top: 1px solid var(--line); }
.live-shodan h3 { font-size: 1rem; margin: 0; }
.live-shodan dl { display: grid; gap: 0.9rem; margin-block: 0.75rem; }
.live-shodan dt, .live-shodan p { font-size: 0.78rem; color: var(--text-muted); }
.live-shodan dd { margin: 0.25rem 0 0; font-size: 0.85rem; overflow-wrap: anywhere; }
.live-shodan summary { color: var(--text-muted); font-size: 0.72rem; cursor: pointer; }
@media (min-width: 640px) { .live-shodan dl { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
@media (min-width: 900px) { .live-ip-facts { grid-template-columns: repeat(3, minmax(0, 1fr)); } }
</style>
