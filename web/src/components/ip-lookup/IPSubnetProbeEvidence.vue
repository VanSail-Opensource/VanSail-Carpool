<script setup lang="ts">
import { computed } from 'vue'
import type { IPLookupReport } from '@/api/ipLookup'
import { visibleProbePorts } from './probePresentation'

const props = defineProps<{ ports: NonNullable<IPLookupReport['ports']> }>()
const findings = computed(() => new Map(visibleProbePorts(props.ports).map(port => [port.port, port.findings])))
</script>

<template>
  <dl class="subnet-probe-evidence" aria-live="polite">
    <div v-for="port in ports" :key="port.port">
      <dt>{{ $t('ipLookup.portLabel', { port: port.port }) }}</dt>
      <dd>
        <span>{{ $t(`ipLookup.probeStatuses.${port.status}`) }}</span>
        <span v-for="finding in findings.get(port.port) ?? []" :key="finding.protocol" :class="`finding-${finding.confidence}`">{{ $t('ipLookup.portService', { protocol: $t(`ipLookup.protocols.${finding.protocol}`), confidence: $t(`ipLookup.confidence.${finding.confidence}`) }) }}<code v-if="finding.dns">{{ finding.dns.domain }}</code></span>
      </dd>
    </div>
  </dl>
</template>

<style scoped>
.subnet-probe-evidence { margin: 0.4rem 0; font-size: 0.75rem; }
.subnet-probe-evidence > div { display: grid; grid-template-columns: minmax(5rem, 1fr) minmax(0, 2fr); gap: 0.4rem; padding-block: 0.25rem; border-bottom: 1px solid var(--line); }
.subnet-probe-evidence dt { color: var(--text-muted); }
.subnet-probe-evidence dd { margin: 0; display: flex; flex-wrap: wrap; gap: 0.2rem 0.6rem; overflow-wrap: anywhere; }
.subnet-probe-evidence code { display: block; color: var(--text-muted); }
.finding-suspected { color: var(--ui-warning); }
.finding-confirmed { color: var(--ui-error); }
</style>
