<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from '@/i18n'
import DemoFlow from '@/components/common/DemoFlow.vue'
import DemoWindow from '@/components/common/DemoWindow.vue'
import { between, ease } from '@/components/common/illustrationTimeline'

const props = defineProps<{ step: number; phase: number; reducedMotion: boolean }>()
const { t } = useI18n()
const p = computed(() => props.reducedMotion ? 1 : props.phase)
const progress = computed(() => ease(between(p.value, .08, .88)))
const ready = computed(() => p.value >= .72)
const monitor = computed(() => props.step >= 4 && props.step <= 6)
const title = computed(() => props.step < 2 ? 'billingChoice' : props.step < 4 ? 'maintained' : monitor.value ? 'quality' : props.step < 9 ? 'privacy' : 'support')
const nodeKeys = computed(() => props.step < 2 ? [['i-ph-wallet-bold', 'account'], ['i-ph-sliders-horizontal-bold', 'billingChoice'], ['i-ph-plugs-connected-bold', 'connection']]
  : props.step < 4 ? [['i-ph-magnifying-glass-bold', 'diagnose'], ['i-ph-wrench-bold', 'recover'], ['i-ph-check-circle-bold', 'maintained']]
    : monitor.value ? [['i-ph-warning-circle-bold', 'issue'], ['i-ph-magnifying-glass-bold', 'diagnose'], ['i-ph-lifebuoy-bold', 'support']]
      : props.step < 9 ? [['i-ph-user-circle-bold', 'account'], ['i-ph-lock-key-bold', 'privacy'], ['i-ph-receipt-bold', 'records']]
        : [['i-ph-devices-bold', 'you'], ['i-ph-chats-circle-bold', 'bot'], ['i-ph-magnifying-glass-bold', 'diagnose']])
const nodes = computed(() => nodeKeys.value.map(([icon, label]) => ({ icon: icon!, label: t(`knowledge.labels.${label}`) })))
const trace = computed(() => props.step === 5 ? 'M 4 55 L 30 46 L 55 51 L 80 38 L 108 71 L 130 75 L 150 70 L 175 73 L 198 76' : 'M 4 55 L 30 46 L 55 51 L 80 38 L 108 71 L 130 42 L 150 29 L 175 33 L 198 24')
const traceProgress = computed(() => props.step === 4 ? .36 + .64 * progress.value : props.step === 6 ? .58 + .42 * progress.value : progress.value)
const completion = (index: number) => ease(between(p.value, .1 + index * .17, .4 + index * .17))
const plans = [['i-ph-clock-bold', 'hourly'], ['i-ph-arrows-down-up-bold', 'traffic']] as const
const report = [['i-ph-hard-drives-bold', 'nodeSelected'], ['i-ph-clock-bold', 'time'], ['i-ph-devices-bold', 'client']] as const
const records = [['i-ph-user-circle-bold', 'account'], ['i-ph-receipt-bold', 'invoice'], ['i-ph-chart-bar-bold', 'traffic']] as const
</script>

<template>
  <DemoWindow class="why-film" :title="t(`knowledge.labels.${title}`)" icon="i-ph-sparkle-bold" :status="t(`knowledge.labels.${step === 5 ? 'diagnose' : step === 7 ? 'transient' : step === 8 ? 'records' : step === 9 ? 'support' : ready ? 'confirmed' : 'pending'}`)" :ready="ready && step !== 5">
    <div class="why-film__content">
      <div v-if="step < 2" class="why-film__plans">
        <div v-for="(plan, index) in plans" :key="plan[1]" class="why-film__plan" :class="{ 'is-selected': step === 1 || (index === 0 ? p < .5 : p >= .5) }">
          <span class="why-film__plan-icon"><UIcon :name="plan[0]" /></span><span>{{ t(`knowledge.labels.${plan[1]}`) }}</span><strong v-if="step === 1">{{ '1.00' }}<small>{{ ' TXB' }}</small></strong>
          <div class="why-film__plan-bars"><i v-for="n in 10" :key="n" :style="{ transform: `scaleY(${index === 0 ? .72 : .25 + ((n * 7) % 8) / 12})`, opacity: n / 10 <= progress ? 1 : .18 }" /></div>
          <small>{{ t(step === 0 ? `knowledge.labels.${index === 0 ? 'time' : 'usage'}` : `knowledge.labels.${index === 0 ? 'perHour' : 'perGiB'}`) }}</small>
        </div>
      </div>
      <div v-else-if="step < 4" class="why-film__project">
        <div class="why-film__code"><UIcon name="i-ph-code-bold" /><div><i v-for="n in 5" :key="n" :style="{ width: `${52 + n % 3 * 16}%`, transform: `scaleX(${ease(between(p, n * .06, .35 + n * .06))})` }" /></div></div>
        <div class="why-film__build"><span>{{ t('knowledge.labels.maintained') }}</span><strong>{{ Math.round(progress * 100) }}{{ '%' }}</strong><span class="why-film__build-bar"><i :style="{ transform: `scaleX(${progress})` }" /></span><UIcon :name="ready ? 'i-ph-check-circle-fill' : 'i-ph-wrench-bold'" /></div>
      </div>
      <div v-else-if="monitor" class="why-film__monitor">
        <div class="why-film__chart-heading"><span><UIcon name="i-ph-heartbeat-bold" />{{ t('knowledge.labels.connection') }}</span><small>{{ t('knowledge.labels.example') }}</small></div>
        <svg class="why-film__chart" viewBox="0 0 204 90" preserveAspectRatio="none">
          <path d="M 4 20 H 200 M 4 45 H 200 M 4 70 H 200" class="why-film__grid" />
          <path :d="trace" class="why-film__trace" pathLength="1" :stroke-dasharray="`${traceProgress} 1`" />
          <circle v-if="step > 4" cx="108" cy="71" r="4" class="why-film__event" :style="{ opacity: progress }" />
        </svg>
        <div class="why-film__incident" :class="{ 'is-reviewing': step === 5 }"><UIcon :name="step === 4 ? 'i-ph-heartbeat-bold' : step === 5 ? 'i-ph-magnifying-glass-bold' : 'i-ph-lifebuoy-bold'" /><span>{{ t(`knowledge.labels.${step === 4 ? 'status' : step === 5 ? 'diagnose' : 'recover'}`) }}</span><i :style="{ transform: `scaleX(${progress})` }" /></div>
      </div>
      <div v-else-if="step < 9" class="why-film__privacy">
        <div class="why-film__transient"><UIcon name="i-ph-plugs-connected-bold" /><span>{{ t('knowledge.labels.connection') }}</span><div><i v-for="n in 7" :key="n" :style="{ opacity: step === 7 ? 1 - between(p, .12 + n * .065, .36 + n * .065) : .1, transform: `translateY(${-5 * between(p, .4, .85)}px)` }" /></div><UIcon name="i-ph-eye-slash-bold" /></div>
        <div class="why-film__records"><span><UIcon name="i-ph-lock-key-bold" />{{ t('knowledge.labels.records') }}</span><div v-for="(record, index) in records" :key="record[1]" :style="{ opacity: step === 8 ? .3 + .7 * completion(index) : .65 }"><UIcon :name="record[0]" /><span>{{ t(`knowledge.labels.${record[1]}`) }}</span><UIcon name="i-ph-check-circle-fill" /></div></div>
      </div>
      <div v-else class="why-film__ticket">
        <span class="why-film__ticket-title"><UIcon name="i-ph-chats-circle-bold" />{{ t('knowledge.labels.bot') }}<small>{{ '#TX-1024' }}</small></span>
        <div v-for="(field, index) in report" :key="field[1]" class="why-film__ticket-field"><UIcon :name="field[0]" /><span>{{ t(`knowledge.labels.${field[1]}`) }}</span><i :style="{ transform: `scaleX(${completion(index)})` }" /><UIcon name="i-ph-check-bold" :style="{ opacity: completion(index) }" /></div>
        <span class="why-film__ticket-send" :style="{ opacity: .35 + .65 * between(p, .65, .9) }"><UIcon name="i-ph-paper-plane-tilt-bold" />{{ t('knowledge.labels.noSecrets') }}</span>
      </div>
    </div>
    <DemoFlow :nodes="nodes" :phase="p" :active="Math.min(2, Math.floor(progress * 3))" :reduced-motion="reducedMotion" />
  </DemoWindow>
</template>

<style scoped>
.why-film__content { display: grid; align-items: center; min-height: 165px; }
.why-film__plans { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: .7rem; }
.why-film__plan { display: grid; grid-template-columns: 1.65rem minmax(0, 1fr); align-items: center; gap: .55rem .4rem; min-width: 0; padding: .75rem; border: 1px solid var(--line); border-radius: .9rem; color: var(--text-muted); font-size: .68rem; }
.why-film__plan.is-selected { border-color: var(--accent-line); background: var(--accent-soft); }
.why-film__plan-icon { display: grid; place-items: center; color: var(--accent); font-size: 1.4rem; }
.why-film__plan strong { grid-column: 1 / -1; font: 1.2rem var(--font-mono); color: var(--text); }
.why-film__plan strong small { font-size: .6rem; color: var(--text-faint); }
.why-film__plan-bars { grid-column: 1 / -1; display: flex; align-items: end; gap: .2rem; height: 2.3rem; }
.why-film__plan-bars i { flex: 1; height: 100%; border-radius: .15rem .15rem 0 0; background: var(--accent); transform-origin: bottom; }
.why-film__plan > small { grid-column: 1 / -1; color: var(--text-faint); font-size: .58rem; }
.why-film__project { display: grid; gap: 1rem; }
.why-film__code { display: flex; align-items: center; gap: 1rem; min-height: 96px; padding: .8rem; border: 1px solid var(--line); border-radius: .8rem; }
.why-film__code > .iconify { flex: none; color: var(--accent); font-size: 2rem; }
.why-film__code > div { display: grid; flex: 1; gap: .5rem; }
.why-film__code i { display: block; height: .3rem; background: var(--accent-line); border-radius: .2rem; transform-origin: left; }
.why-film__code i:nth-child(2n) { margin-inline-start: .75rem; background: var(--text-faint); }
.why-film__build { display: grid; grid-template-columns: 1fr auto auto; gap: .45rem; font-size: .7rem; color: var(--text-muted); }
.why-film__build strong { font: .75rem var(--font-mono); color: var(--text); }
.why-film__build > .iconify { grid-column: 3; grid-row: 1; color: var(--success); font-size: 1.1rem; }
.why-film__build-bar { grid-column: 1 / -1; height: 4px; background: var(--line); border-radius: 4px; overflow: hidden; }
.why-film__build-bar i { display: block; height: 100%; background: var(--accent); transform-origin: left; }
.why-film__chart-heading { display: flex; justify-content: space-between; gap: .6rem; color: var(--text-muted); font-size: .68rem; }
.why-film__chart-heading > span { display: flex; align-items: center; gap: .4rem; }
.why-film__chart-heading .iconify { color: var(--accent); }
.why-film__chart-heading small { font-size: .58rem; color: var(--text-faint); }
.why-film__chart { width: 100%; height: 102px; overflow: visible; }
.why-film__grid { stroke: var(--line); stroke-width: .6; fill: none; }
.why-film__trace { stroke: var(--accent); stroke-width: 2; stroke-linecap: round; stroke-linejoin: round; fill: none; }
.why-film__event { fill: var(--warning); stroke: var(--surface); stroke-width: 2; }
.why-film__incident { display: flex; align-items: center; gap: .45rem; padding: .55rem .65rem; border-radius: .7rem; color: var(--accent); background: var(--accent-soft); font-size: .65rem; }
.why-film__incident.is-reviewing { color: var(--warning); background: var(--warning-soft); }
.why-film__incident i { margin-inline-start: auto; width: 35%; height: 3px; background: currentColor; transform-origin: left; }
.why-film__privacy { display: grid; gap: .7rem; }
.why-film__transient { display: flex; align-items: center; gap: .45rem; min-height: 2rem; color: var(--text-muted); font-size: .67rem; }
.why-film__transient > .iconify { flex: none; color: var(--accent); }
.why-film__transient > div { display: flex; flex: 1; justify-content: end; gap: .25rem; }
.why-film__transient i { width: .38rem; height: .38rem; border-radius: .1rem; background: var(--accent); }
.why-film__records { display: grid; gap: .55rem; padding: .7rem; border: 1px solid var(--line); border-radius: .85rem; }
.why-film__records > span, .why-film__records > div { display: flex; align-items: center; gap: .45rem; font-size: .65rem; color: var(--text-muted); }
.why-film__records > span { color: var(--text); }
.why-film__records .iconify { color: var(--accent); }
.why-film__records > div > .iconify:last-child { margin-inline-start: auto; color: var(--success); }
.why-film__ticket { display: grid; gap: .55rem; }
.why-film__ticket-title { display: flex; gap: .4rem; color: var(--text); font-size: .7rem; }
.why-film__ticket-title .iconify { color: var(--accent); }
.why-film__ticket-title small { margin-inline-start: auto; color: var(--text-faint); font: .6rem var(--font-mono); }
.why-film__ticket-field { display: flex; gap: .45rem; align-items: center; min-height: 1.6rem; color: var(--text-muted); font-size: .65rem; }
.why-film__ticket-field .iconify { color: var(--accent); }
.why-film__ticket-field i { flex: 1; height: 3px; border-radius: 2px; background: var(--accent-line); transform-origin: left; }
.why-film__ticket-send { display: flex; align-items: center; justify-content: center; gap: .4rem; min-height: 2rem; border-radius: .7rem; background: var(--accent-soft); color: var(--accent); font-size: .6rem; }
@media (max-width: 380px) { .why-film__plans { gap: .45rem; } .why-film__plan { padding: .55rem; font-size: .63rem; } .why-film__plan strong { font-size: 1rem; } .why-film__transient { font-size: .6rem; gap: .3rem; } .why-film__transient > div { gap: .15rem; } }
</style>
