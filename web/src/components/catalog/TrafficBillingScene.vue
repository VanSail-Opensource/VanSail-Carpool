<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from '@/i18n'
import { formatBytes } from '@/utils/format'
import DemoPointer from '@/components/common/DemoPointer.vue'
import DemoThreeScene from '@/components/common/DemoThreeScene.vue'
import { between, interactionCue } from '@/components/common/illustrationTimeline'
import { trafficDemoState } from './billingDemo'

const props = defineProps<{ step: number; phase: number; reducedMotion: boolean }>()
const { t } = useI18n()
const frame = computed(() => trafficDemoState(props.step, props.phase))
const cue = computed(() => interactionCue(props.phase, [{ at: props.step === 0 ? .3 : .4, x: 100, y: 50 }]))
const expiry = computed(() => props.step === 5 ? between(props.phase, .06, .62) : 0)
const bytes = (mib: number) => formatBytes(String(mib * 1024 * 1024))
</script>

<template>
  <div class="traffic-scene" aria-hidden="true" :data-demo-blocks="frame.blocks" :data-demo-balance="frame.balance">
    <div class="traffic-scene__top"><span><UIcon name="i-ph-coins" />{{ t('payg.demo.trafficRate') }}</span><span><UIcon name="i-ph-wallet" /><strong>{{ t('payg.demo.txb', { value: frame.balance }) }}</strong></span></div>
    <div class="traffic-scene__connection" :class="{ 'is-connected': frame.connected }">
      <UIcon name="i-ph-computer-tower" /><span>{{ t('payg.demo.node') }}</span><span class="traffic-scene__connect"><UIcon :name="frame.connected ? 'i-ph-check-bold' : 'i-ph-plugs-connected-bold'" /></span>
      <DemoPointer v-if="step === 0 || step === 4" :cue="cue" :reduced-motion="reducedMotion" />
    </div>
    <div class="traffic-scene__cinema">
      <DemoThreeScene topic="traffic" :step="step" :phase="phase" :reduced-motion="reducedMotion" />
      <div class="traffic-scene__stats"><span><UIcon name="i-ph-devices-bold" />{{ bytes(frame.usageMiB) }}</span><span><UIcon name="i-ph-cube-bold" />{{ t('payg.demo.blocks', { count: frame.blocks }) }}</span><span><UIcon name="i-ph-wallet-bold" />{{ t('payg.demo.txb', { value: frame.balance }) }}</span></div>
    </div>
    <div class="traffic-scene__credit" :class="{ 'is-expired': frame.expired }">
      <div class="traffic-scene__credit-heading"><span><UIcon name="i-ph-stack" />{{ t('payg.demo.credit') }}</span><strong>{{ bytes(frame.creditMiB) }}</strong></div>
      <div class="traffic-scene__meter"><div :style="{ transform: `scaleX(${frame.creditMiB / 1024})` }" /></div>
      <div class="traffic-scene__expiry"><UIcon :name="frame.expired ? 'i-ph-hourglass-high' : step === 4 ? 'i-ph-shield-check' : 'i-ph-clock'" /><span>{{ t(step === 4 ? 'payg.demo.creditKept' : 'payg.demo.creditWindow') }}</span><span v-if="step === 5" class="traffic-scene__hours">{{ Math.round(expiry * 72) }}{{ t('payg.demo.hoursUnit') }}</span></div>
    </div>
  </div>
</template>

<style scoped>
.traffic-scene { position: relative; display: grid; gap: .6rem; padding: .8rem; overflow: hidden; --pointer-inset: 2.2rem; }
.traffic-scene__top { display: flex; align-items: center; justify-content: space-between; gap: .5rem; color: var(--text-faint); font: .75rem var(--font-mono); }
.traffic-scene__top > span { display: flex; align-items: center; gap: .35rem; }
.traffic-scene__top strong { color: var(--text); font-weight: 500; }
.traffic-scene__connection { position: relative; display: flex; align-items: center; gap: .6rem; padding: .65rem .9rem; background: var(--surface-raised); border: 1px solid var(--line); border-radius: var(--radius-control); color: var(--text-muted); font-size: .8rem; }
.traffic-scene__connection > .iconify { font-size: 1.35rem; }
.traffic-scene__connect { display: grid; place-items: center; width: 2.6rem; height: 1.7rem; border-radius: .4rem; margin-inline-start: auto; background: var(--surface); border: 1px solid var(--line); }
.is-connected .traffic-scene__connect { color: var(--accent); border-color: var(--accent); background: var(--accent-soft); }
.traffic-scene__cinema { position: relative; min-width: 0; }
.traffic-scene__cinema :deep(.demo-three-scene) { aspect-ratio: 2.8; min-height: 150px; }
.traffic-scene__stats { display: flex; align-items: center; justify-content: space-between; gap: .4rem; padding: .2rem .2rem .4rem; font: .65rem var(--font-mono); color: var(--text-muted); }
.traffic-scene__stats > span { display: flex; align-items: center; gap: .25rem; }
.traffic-scene__stats .iconify { color: var(--accent); flex: none; }
.traffic-scene__credit { display: grid; align-content: center; gap: .7rem; padding: .85rem; border-radius: var(--radius-control); background: var(--surface-raised); }
.traffic-scene__credit-heading { display: flex; align-items: center; justify-content: space-between; gap: .5rem; }
.traffic-scene__credit-heading span { display: inline-flex; align-items: center; gap: .35rem; color: var(--text-muted); font-size: .72rem; }
.traffic-scene__credit-heading strong { font: 500 .9rem var(--font-mono); color: var(--accent); font-variant-numeric: tabular-nums; }
.traffic-scene__meter { height: .45rem; overflow: hidden; background: var(--line); border-radius: .2rem; }
.traffic-scene__meter > div { height: 100%; width: 100%; background: var(--accent); transform-origin: left; }
.traffic-scene__expiry { display: flex; align-items: center; gap: .3rem; color: var(--text-faint); font-size: .64rem; }
.traffic-scene__hours { margin-inline-start: auto; font-family: var(--font-mono); }
.is-expired .traffic-scene__credit-heading strong { color: var(--text-muted); }
</style>
