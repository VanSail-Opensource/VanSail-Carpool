<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from '@/i18n'
import { hourlyDemoFrames } from './billingDemo'

const props = defineProps<{ step: number; playing: boolean }>()
const { t } = useI18n()
const frame = computed(() => hourlyDemoFrames[props.step] ?? hourlyDemoFrames[0])
const poolKey = computed(() => props.step === 3 ? 'reduced' : props.step === 4 || props.step === 5 ? 'extraBlock' : 'dailyPool')
</script>

<template>
  <div class="hourly-scene" :class="{ 'hourly-scene--playing': playing, 'hourly-scene--flowing': playing && (step === 2 || step === 4), 'hourly-scene--reduced': step === 3, 'hourly-scene--extra': step === 4 || step === 5 }" aria-hidden="true">
    <div class="hourly-scene__rate"><UIcon name="i-ph-clock" />{{ t('payg.demo.hourlyRate') }}</div>
    <div class="hourly-scene__ledger">
      <div class="hourly-scene__clock" :style="{ '--clock-fill': step === 0 ? '50%' : '100%' }"><UIcon :name="frame.icon" /></div>
      <div class="hourly-scene__charge"><strong>{{ frame.time }}<small>{{ t('payg.demo.utc') }}</small></strong><span :key="`${step}-debit`">{{ t('payg.demo.debit', { value: frame.debit }) }}</span></div>
      <div class="hourly-scene__wallet"><UIcon name="i-ph-wallet" /><strong :key="frame.balance">{{ t('payg.demo.txb', { value: frame.balance }) }}</strong></div>
    </div>
    <div class="hourly-scene__pool">
      <div class="hourly-scene__squads"><span v-for="index in 3" :key="index" class="hourly-scene__source" :style="{ '--packet-delay': `${index * 300}ms` }"><UIcon name="i-ph-hard-drives" /><i /></span></div>
      <div class="hourly-scene__allowance">
        <div class="hourly-scene__pool-heading"><span>{{ t(`payg.demo.${poolKey}`) }}</span><UIcon :name="step === 3 ? 'i-ph-gauge' : step === 4 ? 'i-ph-rocket-launch' : step === 5 ? 'i-ph-lock-key' : 'i-ph-lightning'" /></div>
        <div class="hourly-scene__meter"><div :style="{ width: `${frame.remaining}%` }" /></div>
        <div class="hourly-scene__note"><UIcon :name="step === 5 ? 'i-ph-hourglass' : step === 6 ? 'i-ph-arrow-counter-clockwise' : 'i-ph-circles-three'" /><span>{{ t(step === 5 ? 'payg.demo.cooldown' : step === 6 ? 'payg.demo.reset' : 'payg.demo.shared') }}</span></div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.hourly-scene { display: grid; gap: 1.3rem; padding: .5rem 0; --pool-color: var(--accent); }
.hourly-scene__rate { display: flex; justify-content: center; align-items: center; gap: .4rem; color: var(--text-faint); font: .75rem var(--font-mono); }
.hourly-scene__ledger { display: flex; align-items: center; gap: .75rem; }
.hourly-scene__clock { display: grid; place-items: center; flex: none; width: 4rem; height: 4rem; border-radius: 50%; background: conic-gradient(var(--accent) var(--clock-fill), var(--line) 0); }
.hourly-scene__clock > .iconify { box-sizing: content-box; width: 1.6rem; height: 1.6rem; padding: 1rem; border-radius: 50%; color: var(--accent); background: var(--surface); }
.hourly-scene__charge { display: grid; gap: .3rem; font: .85rem var(--font-mono); }
.hourly-scene__charge strong { display: flex; flex-wrap: wrap; align-items: baseline; gap: .3rem; }
.hourly-scene__charge small { color: var(--text-faint); font-size: .6rem; }
.hourly-scene__charge > span { color: var(--warning); font-size: .72rem; }
.hourly-scene__wallet { display: grid; gap: .35rem; justify-items: end; margin-inline-start: auto; min-width: 0; }
.hourly-scene__wallet .iconify { color: var(--text-muted); font-size: 1.4rem; }
.hourly-scene__wallet strong { color: var(--text); font: .82rem var(--font-mono); overflow-wrap: anywhere; text-align: right; }
.hourly-scene__pool { display: grid; grid-template-columns: 3rem minmax(0, 1fr); gap: .4rem; }
.hourly-scene__squads { display: grid; align-content: center; justify-content: center; gap: .4rem; color: var(--text-faint); font-size: 1.15rem; }
.hourly-scene__source { display: flex; align-items: center; gap: .25rem; }
.hourly-scene__source > i { position: relative; width: 1.25rem; height: 1px; background: var(--line); }
.hourly-scene__source > i::after { content: ''; position: absolute; top: -2px; left: 0; width: 5px; height: 5px; border-radius: 50%; opacity: 0; background: var(--pool-color); }
.hourly-scene__allowance { display: grid; gap: .6rem; padding: 1rem; border-radius: var(--radius-control); background: var(--surface-raised); }
.hourly-scene__pool-heading { display: flex; align-items: center; justify-content: space-between; gap: .5rem; color: var(--pool-color); font-size: .75rem; }
.hourly-scene__pool-heading .iconify { flex: none; font-size: 1.2rem; }
.hourly-scene__meter { height: .65rem; overflow: hidden; background: var(--line); border-radius: 1rem; }
.hourly-scene__meter > div { height: 100%; background: var(--pool-color); border-radius: inherit; transition: width 600ms ease, background-color 200ms ease; }
.hourly-scene__note { display: flex; justify-content: flex-end; align-items: center; gap: .3rem; color: var(--text-faint); font-size: .67rem; }
.hourly-scene--reduced { --pool-color: var(--text-faint); }
.hourly-scene--extra { --pool-color: var(--success); }
.hourly-scene--playing .hourly-scene__charge > span { animation: hourly-demo-debit 600ms ease; }
.hourly-scene--flowing .hourly-scene__source > i::after { animation: hourly-demo-share 1200ms ease-in-out infinite; animation-delay: var(--packet-delay); }
@keyframes hourly-demo-share { from { left: 0; opacity: 0; } 15% { opacity: 1; } to { left: 100%; opacity: 0; } }
@keyframes hourly-demo-debit { from { opacity: .4; transform: translateY(-3px); } to { opacity: 1; transform: translateY(0); } }
@media (prefers-reduced-motion: reduce) { .hourly-scene__charge > span, .hourly-scene__source > i::after { animation: none !important; } .hourly-scene__meter > div { transition: none; } }
</style>
