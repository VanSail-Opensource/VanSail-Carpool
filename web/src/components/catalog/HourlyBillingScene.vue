<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from '@/i18n'
import DemoPointer from '@/components/common/DemoPointer.vue'
import DemoThreeScene from '@/components/common/DemoThreeScene.vue'
import { interactionCue } from '@/components/common/illustrationTimeline'
import { hourlyDemoState } from './billingDemo'

const props = defineProps<{ step: number; phase: number; reducedMotion: boolean }>()
const { t } = useI18n()
const frame = computed(() => hourlyDemoState(props.step, props.phase))
const cue = computed(() => interactionCue(props.phase, [{ at: props.step === 4 ? .3 : .32, x: 100, y: 50 }]))
const poolKey = computed(() => frame.value.reduced ? 'reduced' : frame.value.extra ? 'extraBlock' : 'dailyPool')
const packet = computed(() => props.reducedMotion ? 50 : (props.phase * 3 % 1) * 100)
const note = computed(() => props.step === 5 ? 'cooldown' : props.step === 6 ? 'reset' : props.step === 1 ? 'idleRenewal' : 'shared')
</script>

<template>
  <div class="hourly-scene" aria-hidden="true" :class="{ 'is-reduced': frame.reduced, 'has-extra': frame.extra, 'is-off': !frame.enabled }" :data-demo-balance="frame.balance" :data-demo-enabled="frame.enabled" :data-demo-auto-buy="frame.autoBuy">
    <div class="hourly-scene__top"><span><UIcon name="i-ph-clock" />{{ t('payg.demo.hourlyRate') }}</span><span><UIcon name="i-ph-wallet" /><strong>{{ t('payg.demo.txb', { value: frame.balance }) }}</strong></span></div>
    <div class="hourly-scene__squad">
      <UIcon name="i-ph-hard-drives" /><span>{{ t('payg.demo.squad') }}</span><span class="hourly-scene__toggle" :class="{ 'is-on': frame.enabled }"><i><UIcon :name="frame.enabled ? 'i-ph-check-bold' : 'i-ph-minus-bold'" /></i></span>
      <DemoPointer v-if="step === 0 || step === 5" :cue="cue" :reduced-motion="reducedMotion" />
    </div>
    <div class="hourly-scene__cinema">
      <DemoThreeScene topic="hourly" :step="step" :phase="phase" :reduced-motion="reducedMotion" />
      <div class="hourly-scene__clock-row">
        <span class="hourly-scene__clock"><UIcon name="i-ph-clock" /><i :style="{ transform: `rotate(${step * 45 + phase * 45}deg)` }" /></span>
        <strong>{{ frame.time }}<small>{{ t('payg.demo.utc') }}</small></strong>
        <span class="hourly-scene__debit" :class="{ 'is-charging': frame.debit !== '0.00' }"><UIcon :name="frame.debit !== '0.00' ? 'i-ph-coins' : 'i-ph-shield-check'" />{{ t('payg.demo.debit', { value: frame.debit }) }}</span>
      </div>
    </div>
    <div class="hourly-scene__allowance">
      <div class="hourly-scene__pool-heading"><span><UIcon :name="frame.reduced ? 'i-ph-gauge' : frame.extra ? 'i-ph-rocket-launch' : 'i-ph-lightning'" />{{ t(`payg.demo.${poolKey}`) }}</span><strong>{{ Math.round(frame.remaining) }}%</strong></div>
      <div class="hourly-scene__meter"><div :style="{ transform: `scaleX(${frame.remaining / 100})` }" /></div>
      <div class="hourly-scene__sources"><span v-for="index in 3" :key="index"><UIcon name="i-ph-hard-drives" /><i><b v-if="step === 2 || (step === 4 && frame.autoBuy)" :style="{ transform: `translateX(${packet}%)` }" /></i></span><small>{{ t(`payg.demo.${note}`) }}</small></div>
    </div>
    <div class="hourly-scene__extra" :class="{ 'is-visible': frame.reduced || step === 4 || step === 5 }">
      <UIcon name="i-ph-rocket-launch" /><span>{{ t('payg.demo.autoBuy') }}</span><span class="hourly-scene__toggle" :class="{ 'is-on': frame.autoBuy }"><i><UIcon :name="frame.autoBuy ? 'i-ph-check-bold' : 'i-ph-minus-bold'" /></i></span>
      <DemoPointer v-if="step === 4" :cue="cue" :reduced-motion="reducedMotion" />
    </div>
  </div>
</template>

<style scoped>
.hourly-scene { position: relative; display: grid; gap: .5rem; padding: .8rem; overflow: hidden; --pool-color: var(--accent); --pointer-inset: 2.2rem; }
.hourly-scene__top { display: flex; align-items: center; justify-content: space-between; gap: .5rem; color: var(--text-faint); font: .75rem var(--font-mono); }
.hourly-scene__top > span { display: flex; align-items: center; gap: .35rem; }
.hourly-scene__top strong { color: var(--text); font-weight: 500; }
.hourly-scene__squad { position: relative; display: flex; align-items: center; gap: .6rem; padding: .65rem .9rem; background: var(--surface-raised); border: 1px solid var(--line); border-radius: var(--radius-control); color: var(--text); font-size: .8rem; }
.hourly-scene__squad > .iconify { color: var(--accent); font-size: 1.35rem; }
.hourly-scene__toggle { display: flex; align-items: center; width: 2.6rem; height: 1.5rem; padding: .15rem; border-radius: .75rem; margin-inline-start: auto; background: var(--line); }
.hourly-scene__toggle i { display: grid; place-items: center; width: 1.2rem; height: 1.2rem; border-radius: 50%; background: var(--surface); color: var(--text-faint); font-size: .75rem; }
.hourly-scene__toggle.is-on { background: var(--accent); }
.hourly-scene__toggle.is-on i { transform: translateX(1.1rem); color: var(--accent); }
.hourly-scene__clock-row { display: flex; align-items: center; gap: .6rem; }
.hourly-scene__cinema { position: relative; min-width: 0; }
.hourly-scene__cinema :deep(.demo-three-scene) { aspect-ratio: 2.8; min-height: 150px; }
.hourly-scene__clock { position: relative; display: grid; place-items: center; width: 2rem; height: 2rem; color: var(--accent); font-size: 1.8rem; }
.hourly-scene__clock > .iconify { opacity: .35; }
.hourly-scene__clock > i { position: absolute; width: 2px; height: .5rem; bottom: 50%; left: calc(50% - 1px); background: var(--accent); transform-origin: bottom; }
.hourly-scene__clock-row > strong { display: flex; align-items: baseline; gap: .25rem; color: var(--text); font: 500 1.05rem var(--font-mono); font-variant-numeric: tabular-nums; }
.hourly-scene__clock-row small { font-size: .6rem; color: var(--text-faint); }
.hourly-scene__debit { display: flex; align-items: center; gap: .3rem; margin-inline-start: auto; color: var(--text-faint); font: .7rem var(--font-mono); }
.hourly-scene__debit.is-charging { color: var(--warning); }
.hourly-scene__allowance { display: grid; align-content: center; gap: .7rem; padding: .85rem; border-radius: var(--radius-control); background: var(--surface-raised); }
.hourly-scene__pool-heading { display: flex; align-items: center; justify-content: space-between; gap: .5rem; color: var(--pool-color); font-size: .72rem; }
.hourly-scene__pool-heading > span { display: inline-flex; align-items: center; gap: .35rem; }
.hourly-scene__pool-heading strong { font: 500 .9rem var(--font-mono); font-variant-numeric: tabular-nums; }
.hourly-scene__meter { height: .45rem; overflow: hidden; background: var(--line); border-radius: .2rem; }
.hourly-scene__meter > div { height: 100%; width: 100%; background: var(--pool-color); transform-origin: left; }
.hourly-scene__sources { display: flex; gap: .2rem; align-items: center; color: var(--text-faint); }
.hourly-scene__sources > span { display: inline-flex; align-items: center; font-size: .85rem; }
.hourly-scene__sources i { width: .6rem; height: 1px; background: var(--line); }
.hourly-scene__sources b { display: block; width: 100%; height: 2px; background: var(--pool-color); }
.hourly-scene__sources > small { margin-inline-start: auto; max-width: 65%; font-size: .61rem; text-align: right; }
.hourly-scene__extra { position: relative; display: flex; align-items: center; gap: .4rem; min-height: 40px; padding-inline: .9rem; font-size: .72rem; color: var(--text-muted); opacity: 0; }
.hourly-scene__extra.is-visible { opacity: 1; }
.hourly-scene__extra > .iconify { color: var(--pool-color); }
.is-reduced { --pool-color: var(--warning); }
.is-off .hourly-scene__squad > .iconify { color: var(--text-faint); }
.has-extra:not(.is-reduced) { --pool-color: var(--success); }
@media (prefers-reduced-motion: no-preference) { .hourly-scene__toggle i { transition: transform 160ms; } .hourly-scene__extra { transition: opacity 180ms; } }
@media (max-width: 380px) { .hourly-scene__clock-row > strong { font-size: .9rem; } }
</style>
