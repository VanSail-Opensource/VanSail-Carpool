<script setup lang="ts">
import { computed, shallowRef, watch } from 'vue'
import type { IPLookupReport } from '@/api/ipLookup'
const props = defineProps<{ result: NonNullable<IPLookupReport['subnet']>; requestedIp: string }>()
const selectedIP = shallowRef('')
const neighbors = computed(() => props.result.addresses.filter(row => row.ip !== props.requestedIp))
const target = computed(() => props.result.addresses.find(row => row.ip === props.requestedIp))
const selected = computed(() => props.result.addresses.find(row => row.ip === selectedIP.value))
watch(() => props.result.prefix, () => { selectedIP.value = '' })
</script>

<template>
  <section class="subnet-report">
    <header><h3>{{ $t('ipLookup.subnetHeading') }}</h3><code>{{ result.prefix }}</code></header>
    <p v-if="result.status === 'skipped'">{{ $t(`ipLookup.subnetReasons.${result.reason}`) }}</p>
    <p v-else-if="result.reason === 'ipv6'">{{ $t('ipLookup.subnetReasons.ipv6') }}</p>
    <template v-else>
      <div class="subnet-legend"><span v-for="status in ['clean', 'flagged', 'unknown']" :key="status"><i :class="`cell-${status}`" />{{ $t(`ipLookup.subnetStates.${status}`) }}</span></div>
      <p v-if="target" class="subnet-target">{{ $t('ipLookup.requestedAddress') }} <code>{{ target.ip }}</code><span>{{ $t(`ipLookup.subnetStates.${target.status}`) }}</span></p>
      <div class="subnet-grid" :aria-label="$t('ipLookup.subnetHeading')">
        <UButton v-for="row in neighbors" :key="row.ip" color="neutral" variant="ghost" size="xs" :class="['subnet-cell', `cell-${row.status}`, { 'cell-selected': selectedIP === row.ip }]" :aria-label="`${row.ip}: ${$t(`ipLookup.subnetStates.${row.status}`)}`" :aria-pressed="selectedIP === row.ip" @click="selectedIP = row.ip">{{ row.ip.split('.').at(-1) }}</UButton>
      </div>
      <dl v-if="selected" class="selected-address" aria-live="polite"><div><dt>{{ $t('ipLookup.address') }}</dt><dd><code>{{ selected.ip }}</code></dd></div><div v-for="source in ['proxy', 'abuse', 'ipapi'] as const" :key="source"><dt>{{ $t(`ipLookup.subnetSources.${source}`) }}</dt><dd>{{ $t(`ipLookup.subnetSourceStates.${selected[source]}`) }}</dd></div></dl>
    </template>
  </section>
</template>

<style scoped>
.subnet-report { border-top: 1px solid var(--line); padding-top: 0.6rem; min-width: 0; }
.subnet-report header { display: flex; flex-wrap: wrap; gap: 0.4rem 0.8rem; align-items: baseline; }
.subnet-report h3 { font-size: 0.85rem; margin: 0; }
.subnet-report p, .subnet-report code, .subnet-legend, .selected-address { font-size: 0.75rem; }
.subnet-report p { color: var(--text-muted); margin: 0.4rem 0; }
.subnet-legend { display: flex; flex-wrap: wrap; gap: 0.7rem; margin-block: 0.4rem; }
.subnet-legend > span { display: inline-flex; align-items: center; gap: 0.3rem; }
.subnet-legend i { width: 0.6rem; height: 0.6rem; }
.subnet-target { display: flex; flex-wrap: wrap; gap: 0.4rem; }
.subnet-grid { display: grid; grid-template-columns: repeat(17, minmax(0, 1fr)); gap: 2px; }
.subnet-cell { display: flex; justify-content: center; padding: 0.22rem 0; min-width: 0; border-radius: 2px; font-size: 0.55rem; line-height: 1.3; }
.cell-clean { background: color-mix(in srgb, var(--ui-success) 30%, var(--surface)); color: var(--ui-success); }
.cell-flagged { background: color-mix(in srgb, var(--ui-error) 30%, var(--surface)); color: var(--ui-error); }
.cell-unknown { background: var(--surface); color: var(--text-muted); }
.cell-selected { outline: 2px solid var(--text); outline-offset: 1px; }
.selected-address { margin: 0.5rem 0; }
.selected-address > div { display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 1fr); padding: 0.2rem 0; border-bottom: 1px solid var(--line); }
.selected-address dt { color: var(--text-muted); }
.selected-address dd { margin: 0; }
@media (min-width: 640px) { .subnet-cell { font-size: 0.7rem; padding-block: 0.35rem; } }
</style>
