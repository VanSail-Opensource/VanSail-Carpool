<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from '@/i18n'
import { between, ease } from '@/components/common/illustrationTimeline'
import { hourlyDemoState } from './billingDemo'
import HourlyDemoAllowanceGraph from './HourlyDemoAllowanceGraph.vue'

const props = defineProps<{ step: number; phase: number; reducedMotion: boolean }>()
const { t } = useI18n()
const frame = computed(() => hourlyDemoState(props.step, props.phase))
const emphasizeExtra = computed(() => props.step >= 5 && props.step <= 7)
const timeline = computed(() => props.step === 0 ? 0 : props.step === 1 ? .5 * ease(between(props.phase, .1, .7)) : props.step === 2 ? .5 + .5 * ease(between(props.phase, .1, .62)) : 1)
const note = computed(() => props.step === 0 ? 'eligibilityCheck' : props.step === 2 ? 'idleRenewal' : props.step === 7 ? 'paidCreditKept' : props.step === 8 ? 'cooldown' : props.step === 9 ? 'reset' : 'perSquadFee')
</script>

<template>
  <div class="hourly-scene" aria-hidden="true" data-demo-visual="hourly" :data-demo-frame="`${step}:${phase.toFixed(3)}`" :data-demo-balance="frame.balance" :data-demo-enabled="frame.enabled" :data-demo-auto-buy="frame.autoBuy">
    <div class="hourly-scene__top"><span><UIcon name="i-ph-clock-bold" />{{ t('payg.demo.hourlyRate') }}</span><span><UIcon name="i-ph-wallet-bold" /><strong>{{ t('payg.demo.txb', { value: frame.balance }) }}</strong></span></div>
    <div class="hourly-scene__squad"><UIcon name="i-ph-hard-drives-bold" /><span>{{ t('payg.demo.squad') }}</span><span class="hourly-scene__toggle" :class="{ 'is-on': frame.enabled }"><i><UIcon :name="frame.enabled ? 'i-ph-check-bold' : 'i-ph-minus-bold'" /></i></span></div>
    <div class="hourly-scene__receipt">
      <div class="hourly-scene__clock-row"><UIcon :name="step === 2 ? 'i-ph-moon-bold' : step === 9 ? 'i-ph-calendar-check-bold' : 'i-ph-clock-bold'" /><strong>{{ frame.time }}<small>{{ t('payg.demo.utc') }}</small></strong><span :class="{ 'is-charging': frame.debit !== '0.00' }"><UIcon :name="frame.debit !== '0.00' ? 'i-ph-coins-bold' : 'i-ph-shield-check-bold'" />{{ t('payg.demo.debit', { value: frame.debit }) }}</span></div>
      <div class="hourly-scene__timeline"><i :style="{ transform: `scaleX(${timeline})` }" /><b v-for="index in 3" :key="index" /></div>
      <div class="hourly-scene__ticks"><span>{{ t('payg.demo.firstHalf') }}</span><span>{{ t('payg.demo.nextHour') }}</span></div>
    </div>
    <HourlyDemoAllowanceGraph :remaining="frame.remaining" :enabled="frame.enabled" :extra="frame.extra" :reduced="frame.reduced" :phase="phase" :moving="[3, 4, 6, 7].includes(step)" :reduced-motion="reducedMotion" />
    <div class="hourly-scene__extra" :class="{ 'is-emphasized': emphasizeExtra }"><UIcon name="i-ph-rocket-launch-bold" /><span>{{ t('payg.demo.autoBuy') }}</span><span class="hourly-scene__toggle" :class="{ 'is-on': frame.autoBuy }"><i><UIcon :name="frame.autoBuy ? 'i-ph-check-bold' : 'i-ph-minus-bold'" /></i></span></div>
    <div class="hourly-scene__note"><UIcon :name="step === 8 ? 'i-ph-hourglass-bold' : 'i-ph-info-bold'" /><span>{{ t(`payg.demo.${note}`) }}</span></div>
  </div>
</template>

<style scoped>
.hourly-scene { display: grid; gap: .65rem; min-width: 0; padding: .85rem; overflow: hidden; }
.hourly-scene__top { display: flex; align-items: center; justify-content: space-between; gap: .5rem; color: var(--text-faint); font: .7rem var(--font-mono); }
.hourly-scene__top > span { display: flex; align-items: center; gap: .35rem; min-width: 0; }
.hourly-scene__top strong { color: var(--text); font-weight: 500; }
.hourly-scene__squad, .hourly-scene__extra { display: flex; align-items: center; gap: .5rem; min-width: 0; padding: .65rem .8rem; background: var(--surface-raised); border: 1px solid var(--line); border-radius: var(--radius-control); color: var(--text); font-size: .75rem; }
.hourly-scene__squad > .iconify, .hourly-scene__extra > .iconify { flex: none; color: var(--accent); font-size: 1.15rem; }
.hourly-scene__toggle { display: flex; flex: none; align-items: center; width: 2.6rem; height: 1.5rem; padding: .15rem; border-radius: 1rem; margin-inline-start: auto; background: var(--line); }
.hourly-scene__toggle i { display: grid; place-items: center; width: 1.2rem; height: 1.2rem; border-radius: 50%; background: var(--surface); color: var(--text-faint); font-size: .75rem; }
.hourly-scene__toggle.is-on { background: var(--accent); }
.hourly-scene__toggle.is-on i { transform: translateX(1.1rem); color: var(--accent); }
.hourly-scene__receipt { display: grid; gap: .6rem; padding: .65rem .2rem .3rem; }
.hourly-scene__clock-row { display: flex; align-items: center; gap: .5rem; min-width: 0; }
.hourly-scene__clock-row > .iconify { flex: none; font-size: 1.3rem; color: var(--accent); }
.hourly-scene__clock-row > strong { display: flex; align-items: baseline; gap: .3rem; color: var(--text); font: 500 1.2rem var(--font-mono); font-variant-numeric: tabular-nums; }
.hourly-scene__clock-row small { color: var(--text-faint); font-size: .6rem; }
.hourly-scene__clock-row > span { display: flex; align-items: center; gap: .3rem; margin-inline-start: auto; color: var(--text-faint); font: .72rem var(--font-mono); }
.hourly-scene__clock-row > span.is-charging { color: var(--warning); }
.hourly-scene__timeline { position: relative; display: flex; justify-content: space-between; height: .3rem; border-radius: 1rem; background: var(--line); }
.hourly-scene__timeline > i { position: absolute; inset: 0; border-radius: inherit; background: var(--accent); transform-origin: left; }
.hourly-scene__timeline > b { z-index: 1; width: .3rem; height: .3rem; border-radius: 50%; background: var(--text-muted); }
.hourly-scene__ticks { display: flex; justify-content: space-between; gap: .5rem; color: var(--text-faint); font-size: .58rem; }
.hourly-scene__extra { color: var(--text-muted); background: transparent; }
.hourly-scene__extra.is-emphasized { border-color: color-mix(in srgb, var(--accent) 45%, var(--line)); }
.hourly-scene__note { display: flex; justify-content: center; align-items: center; gap: .4rem; min-height: 1.5rem; color: var(--text-faint); font-size: .63rem; text-align: center; }
.hourly-scene__note > .iconify { flex: none; color: var(--accent); }
@media (max-width: 380px) { .hourly-scene { padding: .65rem; } .hourly-scene__top { font-size: .62rem; } .hourly-scene__clock-row > strong { font-size: 1.05rem; } }
</style>
