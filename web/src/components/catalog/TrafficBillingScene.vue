<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from '@/i18n'
import { formatBytes } from '@/utils/format'
import { trafficDemoFrames } from './billingDemo'

const props = defineProps<{ step: number; playing: boolean }>()
const { t } = useI18n()
const frame = computed(() => trafficDemoFrames[props.step] ?? trafficDemoFrames[0])
const flowing = computed(() => props.step > 0 && props.step < 4)
const bytes = (mib: number) => formatBytes(String(mib * 1024 * 1024))
</script>

<template>
  <div class="traffic-scene" :class="{ 'traffic-scene--moving': playing && flowing, 'traffic-scene--buying': playing && (step === 1 || step === 3) }" aria-hidden="true">
    <div class="traffic-scene__rate"><UIcon name="i-ph-coins" />{{ t('payg.demo.trafficRate') }}</div>
    <div class="traffic-scene__flow">
      <div class="traffic-scene__node"><UIcon :name="step === 4 ? 'i-ph-arrows-clockwise' : 'i-ph-devices'" /><span>{{ bytes(frame.usageMiB) }}</span></div>
      <div class="traffic-scene__track"><i /><i /></div>
      <div class="traffic-scene__node traffic-scene__node--credit"><UIcon :name="step === 5 ? 'i-ph-hourglass' : 'i-ph-cube'" /><span>{{ t('payg.demo.blocks', { count: frame.blocks }) }}</span></div>
      <div class="traffic-scene__track traffic-scene__track--coins"><i /></div>
      <div class="traffic-scene__node"><UIcon name="i-ph-wallet" /><span :key="frame.balance" class="traffic-scene__amount">{{ t('payg.demo.txb', { value: frame.balance }) }}</span></div>
    </div>
    <div class="traffic-scene__credit">
      <div class="traffic-scene__credit-heading"><span><UIcon name="i-ph-stack" />{{ t('payg.demo.credit') }}</span><strong>{{ bytes(frame.creditMiB) }}</strong></div>
      <div class="traffic-scene__meter"><div :style="{ width: `${frame.creditMiB * 100 / 1024}%` }" /><span v-for="tick in 7" :key="tick" :style="{ left: `${tick * 12.5}%` }" /></div>
      <div class="traffic-scene__expiry"><UIcon :name="step === 5 ? 'i-ph-hourglass-high' : 'i-ph-clock'" /><span>{{ t('payg.demo.creditWindow') }}</span><UIcon v-if="step === 4" name="i-ph-check-circle" /></div>
    </div>
  </div>
</template>

<style scoped>
.traffic-scene { display: grid; gap: 1.6rem; padding: .5rem 0; }
.traffic-scene__rate { display: flex; justify-content: center; align-items: center; gap: .4rem; color: var(--text-faint); font: .75rem var(--font-mono); }
.traffic-scene__flow { display: grid; grid-template-columns: minmax(0, 1fr) .4fr minmax(0, 1fr) .4fr minmax(0, 1fr); align-items: start; }
.traffic-scene__node { display: grid; justify-items: center; gap: .6rem; min-width: 0; color: var(--text-muted); }
.traffic-scene__node > .iconify { box-sizing: content-box; width: 1.8rem; height: 1.8rem; padding: .9rem; border: 1px solid var(--line); border-radius: 1rem; background: var(--surface-raised); }
.traffic-scene__node--credit > .iconify { color: var(--accent); border-color: color-mix(in srgb, var(--accent) 30%, var(--line)); background: var(--accent-soft); }
.traffic-scene__node > span { font: .72rem var(--font-mono); text-align: center; overflow-wrap: anywhere; }
.traffic-scene__track { position: relative; height: 2px; margin-top: 1.8rem; background: var(--line); }
.traffic-scene__track i { position: absolute; top: -2px; left: 0; width: 6px; height: 6px; border-radius: 50%; background: var(--accent); opacity: 0; animation: demo-packet 1200ms ease-in-out infinite; animation-play-state: paused; }
.traffic-scene__track i:nth-child(2) { animation-delay: 600ms; }
.traffic-scene__track--coins i { background: var(--warning); }
.traffic-scene--moving .traffic-scene__track:not(.traffic-scene__track--coins) i { animation-play-state: running; }
.traffic-scene--buying .traffic-scene__track--coins i { animation-play-state: running; animation-iteration-count: 1; }
.traffic-scene__credit { display: grid; gap: .6rem; padding: 1rem; border-radius: var(--radius-control); background: var(--surface-raised); }
.traffic-scene__credit-heading { display: flex; align-items: center; justify-content: space-between; gap: .5rem; }
.traffic-scene__credit-heading span { display: inline-flex; align-items: center; gap: .35rem; color: var(--text-muted); font-size: .72rem; }
.traffic-scene__credit-heading strong { font: 600 .9rem var(--font-mono); color: var(--accent); }
.traffic-scene__meter { position: relative; height: .65rem; overflow: hidden; background: var(--line); border-radius: 1rem; }
.traffic-scene__meter > div { height: 100%; background: var(--accent); border-radius: inherit; transition: width 600ms ease; }
.traffic-scene__meter > span { position: absolute; top: 0; bottom: 0; width: 2px; background: var(--surface-raised); }
.traffic-scene__expiry { display: flex; align-items: center; justify-content: flex-end; gap: .3rem; color: var(--text-faint); font-size: .67rem; }
.traffic-scene--moving .traffic-scene__amount { animation: demo-amount 380ms ease-out; }
@keyframes demo-packet { from { left: 0; opacity: 0; } 15% { opacity: 1; } to { left: 100%; opacity: 0; } }
@keyframes demo-amount { from { color: var(--warning); transform: translateY(-3px); } to { color: var(--text-muted); transform: translateY(0); } }
@media (prefers-reduced-motion: reduce) { .traffic-scene__track i, .traffic-scene__amount { animation: none !important; } .traffic-scene__meter > div { transition: none; } }
</style>
