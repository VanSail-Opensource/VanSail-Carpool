<script setup lang="ts">
import { computed, shallowRef, watch } from 'vue'
import type { IPLookupASNDetails, IPLookupReport } from '@/api/ipLookup'
import LiveSourceNote from './LiveSourceNote.vue'
import IPTrafficBars from './IPTrafficBars.vue'
import IPRPKIStatus from './IPRPKIStatus.vue'
const props = defineProps<{ asns: IPLookupASNDetails[]; rpki?: IPLookupReport['rpki'] }>()
const selected = shallowRef<number | null>(null)
watch(() => props.asns, value => { if (!value.some(asn => asn.asn === selected.value)) selected.value = value[0]?.asn ?? null }, { immediate: true })
const current = computed(() => props.asns.find(asn => asn.asn === selected.value))
const rows = computed(() => current.value ? [
  { key: 'registeredAt', value: current.value.registeredAt ? new Date(current.value.registeredAt).toLocaleDateString() : null, source: 'registration' },
  { key: 'rir', value: current.value.rir || null, source: 'rir' },
  { key: 'ipv4Prefixes', value: current.value.ipv4Prefixes, source: current.value.sources.ipv4Prefixes ? 'ipv4Prefixes' : 'routing' },
  { key: 'ipv6Prefixes', value: current.value.ipv6Prefixes, source: current.value.sources.ipv6Prefixes ? 'ipv6Prefixes' : 'routing' },
  { key: 'observedNeighbors', value: current.value.observedNeighbors, source: 'routing' },
  { key: 'peers', value: current.value.peers, source: 'relationships' },
] : [])
</script>

<template>
  <section v-if="current" class="asn-details">
    <div class="asn-heading"><h3>{{ $t('ipLookupLive.asnTitle') }}</h3><USelect v-if="asns.length > 1" v-model="selected" :items="asns.map(asn => ({ label: `AS${asn.asn}`, value: asn.asn }))" :aria-label="$t('ipLookupLive.selectOrigin')" /></div>
    <p class="asn-identity"><code>AS{{ current.asn }}</code><span>{{ current.name }}</span><span class="asn-routing">{{ current.announced === null ? $t('ipLookupLive.status.empty') : current.announced ? $t('ipLookupLive.announced') : $t('ipLookupLive.notAnnounced') }}</span></p>
    <dl class="asn-facts"><div v-for="row in rows" :key="row.key"><dt>{{ $t(`ipLookupLive.fields.${row.key}`) }}</dt><dd>{{ row.value ?? $t('ipLookupLive.status.empty') }}<LiveSourceNote v-if="current.sources[row.source]" :meta="current.sources[row.source]!" /></dd></div></dl>
    <div class="asn-routing-summary"><dl><dt>{{ $t('ipLookupLive.fields.upstreams') }}</dt><dd>{{ current.upstreams ?? $t('ipLookupLive.status.empty') }}<LiveSourceNote v-if="current.sources.relationships" :meta="current.sources.relationships" /></dd></dl><IPRPKIStatus :result="rpki" compact /></div>
    <dl class="asn-contacts"><div><dt>{{ $t('ipLookupLive.fields.abuseContacts') }}</dt><dd><a v-for="contact in current.abuseContacts" :key="contact" :href="`mailto:${contact}`">{{ contact }}</a><span v-if="!current.abuseContacts?.length">{{ current.sources.abuseContacts ? $t('ipLookupLive.status.empty') : $t('ipLookupLive.refreshRequired') }}</span><LiveSourceNote v-if="current.sources.abuseContacts" :meta="current.sources.abuseContacts" /></dd></div><div><dt>{{ $t('ipLookupLive.fields.website') }}</dt><dd><a v-if="current.website" :href="current.website" target="_blank" rel="noopener noreferrer">{{ current.website }}</a><span v-else>{{ current.sources.website ? $t('ipLookupLive.status.empty') : $t('ipLookupLive.refreshRequired') }}</span><LiveSourceNote v-if="current.sources.website" :meta="current.sources.website" /></dd></div></dl>
    <div class="asn-traffic"><IPTrafficBars :summary="current.traffic.botHuman" :title="$t('ipLookupLive.traffic.botHuman')" /><IPTrafficBars :summary="current.traffic.devices" :title="$t('ipLookupLive.traffic.devices')" /><IPTrafficBars :summary="current.traffic.ipVersion" :title="$t('ipLookupLive.traffic.ipVersion')" /></div>
    <details class="asn-context"><summary>{{ $t('ipLookupLive.about') }}</summary><p>{{ $t('ipLookupLive.traffic.context') }}</p></details>
  </section>
</template>

<style scoped>
.asn-details { padding-block: 0.75rem; border-top: 1px solid var(--line); }
.asn-heading { display: flex; flex-wrap: wrap; justify-content: space-between; align-items: center; gap: 0.6rem; }
.asn-heading h3 { font-size: 1rem; margin: 0; }
.asn-identity { display: flex; flex-wrap: wrap; align-items: baseline; gap: 0.4rem 0.7rem; margin: 0.75rem 0 1rem; font-size: 0.85rem; overflow-wrap: anywhere; }
.asn-routing, .asn-facts dt, .asn-context { color: var(--text-muted); font-size: 0.78rem; }
.asn-facts { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 0.6rem; margin: 0; }
.asn-facts > div { min-width: 0; }
.asn-facts dd { margin: 0.25rem 0 0; font-size: 0.88rem; overflow-wrap: anywhere; }
.asn-routing-summary { display: grid; gap: 0.6rem; align-items: start; margin-top: 0.6rem; }
.asn-routing-summary dl { margin: 0; min-width: 0; }
.asn-routing-summary dt { font-size: 0.78rem; color: var(--text-muted); }
.asn-routing-summary dd { margin: 0.25rem 0 0; font-size: 0.88rem; }
@media (min-width: 640px) { .asn-routing-summary { grid-template-columns: minmax(0, 1fr) minmax(0, 2fr); } }
.asn-traffic { display: grid; gap: 0.65rem; margin-top: 0.8rem; }
.asn-contacts { display: grid; gap: 0.6rem; margin-block: 0.8rem; }
.asn-contacts dt { color: var(--text-muted); font-size: 0.78rem; }
.asn-contacts dd { margin: 0.25rem 0 0; font-size: 0.82rem; overflow-wrap: anywhere; }
.asn-contacts a { display: block; color: var(--ui-info); }
@media (min-width: 640px) { .asn-contacts { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
.asn-context { line-height: 1.5; margin-bottom: 0; }
@media (min-width: 900px) { .asn-facts { grid-template-columns: repeat(4, minmax(0, 1fr)); } .asn-traffic { grid-template-columns: repeat(3, minmax(0, 1fr)); } }
</style>
