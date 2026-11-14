<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from '@/i18n'
import { formatBytes } from '@/utils/format'
import { between } from '@/components/common/illustrationTimeline'
import { trafficDemoState } from './billingDemo'
import TrafficUsageGraph from './TrafficUsageGraph.vue'

const props = defineProps<{ step: number; phase: number; reducedMotion: boolean }>()
const { t } = useI18n()
const frame = computed(() => trafficDemoState(props.step, props.phase))
const expiry = computed(() => props.step === 7 ? between(props.phase, .06, .62) : props.step > 7 ? 1 : 0)
const bytes = (mib: number) => formatBytes(String(mib * 1024 * 1024))
const contextualKey = computed(() => props.step === 0 ? 'addBalance' : props.step === 6 ? 'separateCredit' : props.step === 8 ? 'reviewCosts' : props.step === 5 ? 'creditKept' : props.step === 7 ? 'noExpiryCharge' : 'uploadDownload')
</script>

<template>
  <div class="traffic-scene" aria-hidden="true" data-demo-visual="traffic" :data-demo-frame="`${step}:${phase.toFixed(3)}`" :data-demo-blocks="frame.blocks" :data-demo-balance="frame.balance">
    <div class="traffic-scene__top"><span><UIcon name="i-ph-coins-bold" />{{ t('payg.demo.trafficRate') }}</span><span><UIcon name="i-ph-wallet-bold" /><strong>{{ t('payg.demo.txb', { value: frame.balance }) }}</strong></span></div>
    <div class="traffic-scene__connection" :class="{ 'is-connected': frame.connected }">
      <UIcon name="i-ph-hard-drives-bold" /><span>{{ t('payg.demo.node') }}</span><small>{{ t(`payg.demo.${frame.connected ? 'connected' : 'ready'}`) }}</small><UIcon :name="frame.connected ? 'i-ph-check-circle-fill' : 'i-ph-plugs-bold'" />
    </div>
    <TrafficUsageGraph :usage-mi-b="frame.usageMiB" :blocks="frame.blocks" :expired="frame.expired" :phase="phase" :moving="[2, 3, 4].includes(step)" :reduced-motion="reducedMotion" />
    <div class="traffic-scene__credit" :class="{ 'is-expired': frame.expired }">
      <div class="traffic-scene__credit-heading"><span><UIcon name="i-ph-stack-bold" />{{ t('payg.demo.credit') }}</span><strong>{{ bytes(frame.creditMiB) }}</strong></div>
      <div class="traffic-scene__meter"><div :style="{ transform: `scaleX(${frame.creditMiB / 1024})` }" /></div>
      <div class="traffic-scene__expiry"><UIcon :name="frame.expired ? 'i-ph-hourglass-high-bold' : 'i-ph-clock-bold'" /><span>{{ t('payg.demo.creditWindow') }}</span><span v-if="step >= 7" class="traffic-scene__hours">{{ Math.round(expiry * 72) }}{{ t('payg.demo.hoursUnit') }}</span></div>
    </div>
    <div class="traffic-scene__context">
      <template v-if="step === 6"><UIcon name="i-ph-hard-drives-bold" /><span>{{ t('payg.demo.nodeB') }}</span><strong>{{ bytes(0) }}</strong><UIcon name="i-ph-not-equals-bold" /><span>{{ t(`payg.demo.${contextualKey}`) }}</span></template>
      <template v-else-if="step === 8"><UIcon name="i-ph-receipt-bold" /><span>{{ t('payg.demo.blocks', { count: frame.blocks }) }}</span><UIcon name="i-ph-equals-bold" /><strong>{{ t('payg.demo.txb', { value: '2.00' }) }}</strong></template>
      <template v-else><UIcon :name="step === 0 ? 'i-ph-plus-circle-bold' : 'i-ph-shield-check-bold'" /><span>{{ t(`payg.demo.${contextualKey}`) }}</span><UIcon v-if="step === 0 && frame.funded" name="i-ph-check-circle-fill" /></template>
    </div>
  </div>
</template>

<style scoped>
.traffic-scene { display: grid; gap: .65rem; min-width: 0; padding: .85rem; overflow: hidden; }
.traffic-scene__top { display: flex; align-items: center; justify-content: space-between; gap: .5rem; color: var(--text-faint); font: .7rem var(--font-mono); }
.traffic-scene__top > span { display: flex; align-items: center; gap: .35rem; min-width: 0; }
.traffic-scene__top strong { color: var(--text); font-weight: 500; }
.traffic-scene__connection { display: flex; align-items: center; gap: .55rem; padding: .65rem .8rem; background: var(--surface-raised); border: 1px solid var(--line); border-radius: var(--radius-control); color: var(--text-muted); font-size: .75rem; }
.traffic-scene__connection > .iconify { flex: none; font-size: 1.1rem; }
.traffic-scene__connection > small { margin-inline-start: auto; font-size: .65rem; }
.is-connected > .iconify:last-child { color: var(--success); }
.traffic-scene__credit { display: grid; gap: .6rem; padding: .8rem; border-radius: var(--radius-control); background: var(--surface-raised); }
.traffic-scene__credit-heading { display: flex; align-items: center; justify-content: space-between; gap: .5rem; }
.traffic-scene__credit-heading span { display: inline-flex; align-items: center; gap: .35rem; color: var(--text-muted); font-size: .7rem; }
.traffic-scene__credit-heading strong { color: var(--accent); font: 500 .9rem var(--font-mono); font-variant-numeric: tabular-nums; }
.traffic-scene__meter { height: .4rem; overflow: hidden; border-radius: 1rem; background: var(--line); }
.traffic-scene__meter > div { height: 100%; width: 100%; background: var(--accent); transform-origin: left; }
.traffic-scene__expiry { display: flex; align-items: center; gap: .3rem; color: var(--text-faint); font-size: .63rem; }
.traffic-scene__hours { margin-inline-start: auto; font-family: var(--font-mono); }
.is-expired .traffic-scene__credit-heading strong { color: var(--text-muted); }
.traffic-scene__context { display: flex; align-items: center; justify-content: center; flex-wrap: wrap; gap: .4rem; min-height: 2rem; color: var(--text-muted); font-size: .66rem; }
.traffic-scene__context > .iconify { flex: none; color: var(--accent); }
.traffic-scene__context > strong { color: var(--text); font-family: var(--font-mono); font-weight: 500; }
@media (max-width: 380px) { .traffic-scene { padding: .65rem; } .traffic-scene__top { font-size: .62rem; } }
</style>
