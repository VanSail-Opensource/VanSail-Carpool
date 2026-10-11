<script setup lang="ts">
import type { IPLookupReport } from '@/api/ipLookup'
defineProps<{ ports: NonNullable<IPLookupReport['ports']>; sourceIp: string }>()
const elapsed = (started: string) => Math.max(0, Math.floor((Date.now() - Date.parse(started)) / 1000))
</script>

<template>
  <section class="port-probes">
    <h3>{{ $t('ipLookup.portHeading') }} <code>{{ sourceIp }}</code></h3>
    <div v-for="port in ports" :key="port.port" class="port-row">
      <code>{{ port.port }}</code>
      <div class="port-evidence">
        <span>{{ $t(`ipLookup.probeStatuses.${port.status}`) }}</span>
        <span v-for="(finding, index) in port.findings.filter(item => item.confidence !== 'observed')" :key="index" :class="`finding-${finding.confidence}`">{{ $t(`ipLookup.protocols.${finding.protocol}`) }} · {{ $t(`ipLookup.confidence.${finding.confidence}`) }}<small>{{ $t(`ipLookup.probeEvidence.${finding.evidence}`) }}</small></span>
        <p v-for="task in port.tasks.filter(item => item.status === 'active')" :key="task.key" class="probe-active">{{ $t(`ipLookup.probes.${task.key}`) }}<span v-if="task.startedAt">{{ $t('ipLookup.probeElapsed', { seconds: elapsed(task.startedAt) }) }}</span><span v-if="task.nextMilestone">{{ $t('ipLookup.probeMilestone', { seconds: task.nextMilestone }) }}</span></p>
        <details><summary>{{ $t('ipLookup.probeCoverage') }}</summary><dl><div v-for="task in port.tasks" :key="task.key"><dt>{{ $t(`ipLookup.probes.${task.key}`) }}</dt><dd>{{ $t(`ipLookup.probeStatuses.${task.status}`) }}</dd></div></dl></details>
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
.port-evidence details { width: 100%; color: var(--text-muted); }
.port-evidence summary { cursor: pointer; }
.port-evidence dl { margin: 0.3rem 0; }
.port-evidence dl > div { display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: 0.5rem; }
.port-evidence dd { margin: 0; }
.probe-active { display: flex; flex-wrap: wrap; gap: 0.3rem; margin: 0; width: 100%; }
.finding-confirmed { color: var(--ui-error); }
.finding-suspected { color: var(--warning); }
</style>
