<script setup lang="ts">
import type { IPLookupReport } from '@/api/ipLookup'
defineProps<{ ports: NonNullable<IPLookupReport['ports']>; sourceIp: string }>()
const elapsed = (started: string) => Math.max(0, Math.floor((Date.now() - Date.parse(started)) / 1000))
</script>

<template>
  <section class="port-probes">
    <h3>{{ $t('ipLookup.portHeading') }} <code>{{ sourceIp }}</code></h3>
    <div class="ports-grid">
      <div v-for="port in ports" :key="port.port" class="port-row">
        <code>{{ port.port }}</code>
        <div class="port-evidence">
          <span>{{ $t(`ipLookup.probeStatuses.${port.status}`) }}</span>
          <span v-for="(finding, index) in port.findings.filter(item => item.protocol !== 'tls')" :key="index" :class="`finding-${finding.confidence}`">{{ $t(`ipLookup.protocols.${finding.protocol}`) }} · {{ finding.protocol === 'web_server' ? $t('ipLookup.verdicts.suitable') : finding.evidence === 'dns_no_address' ? $t('ipLookup.probeStatuses.skipped') : $t(`ipLookup.confidence.${finding.confidence}`) }}<small>{{ $t(`ipLookup.probeEvidence.${finding.evidence}`) }}</small><small v-if="finding.dns?.domain"><code>{{ finding.dns.domain }}</code><span v-for="resolver in finding.dns.resolvers" :key="resolver.server" class="dns-evidence">{{ resolver.server }}: {{ $t(`ipLookup.dnsStatuses.${resolver.status}`) }}<code v-if="resolver.addresses.length">{{ resolver.addresses.join(', ') }}</code></span></small></span>
          <p v-for="task in port.tasks.filter(item => item.status === 'active')" :key="task.key" class="probe-active">{{ $t(`ipLookup.probes.${task.key}`) }}<span v-if="task.startedAt">{{ $t('ipLookup.probeElapsed', { seconds: elapsed(task.startedAt) }) }}</span><span v-if="task.nextMilestone">{{ $t('ipLookup.probeMilestone', { seconds: task.nextMilestone }) }}</span></p>
          <dl class="probe-tasks"><div v-for="task in port.tasks" :key="task.key"><dt>{{ $t(`ipLookup.probes.${task.key}`) }}</dt><dd>{{ $t(`ipLookup.probeStatuses.${task.status}`) }}<small v-if="task.result">{{ $t(`ipLookup.probeResults.${task.result}`) }}</small></dd></div></dl>
        </div>
      </div>
    </div>
  </section>
</template>

<style scoped>
.port-probes { border-top: 1px solid var(--line); padding-top: 0.6rem; }
.port-probes h3 { font-size: 0.85rem; margin: 0 0 0.4rem; display: flex; flex-wrap: wrap; gap: 0.5rem; }
.port-probes h3 code { color: var(--text-muted); font-weight: normal; }
.port-row { display: grid; grid-template-columns: 3.2rem minmax(0, 1fr); gap: 0.6rem; padding: 0.45rem 0; border-bottom: 1px solid var(--line); font-size: 0.78rem; }
.port-evidence { display: flex; flex-wrap: wrap; gap: 0.3rem 0.7rem; min-width: 0; }
.port-evidence small { display: block; color: var(--text-muted); }
.probe-tasks { width: 100%; color: var(--text-muted); }
.port-evidence dl { margin: 0.3rem 0; }
.port-evidence dl > div { display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: 0.5rem; }
.port-evidence dd { margin: 0; text-align: right; }
.probe-active { display: flex; flex-wrap: wrap; gap: 0.3rem; margin: 0; width: 100%; }
.finding-confirmed { color: var(--ui-error); }
.finding-suspected { color: var(--warning); }
.ports-grid { display: grid; gap: 0 1rem; }
.dns-evidence { display: grid; gap: 0.15rem; margin-top: 0.25rem; overflow-wrap: anywhere; }
@media (min-width: 900px) { .ports-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
@media (min-width: 1200px) { .ports-grid { grid-template-columns: repeat(3, minmax(0, 1fr)); } }
</style>
