<script setup lang="ts">
import { computed, onScopeDispose, shallowRef, watch } from 'vue'
import { motion } from 'motion-v'

import type { HourlyView } from '@/api/hourly'
import { useMotionPreferences } from '@/composables/useMotionPreferences'
import { useI18n } from '@/i18n'
import { formatBytes } from '@/utils/format'
import HourlyThrottleMeter from './HourlyThrottleMeter.vue'
import { hourlyAllowance } from './hourlyAllowance'

type Usage = HourlyView['usage']
type Policy = HourlyView['policy']['policy']

const props = withDefaults(defineProps<{ usage: Usage; policy: Policy; extraEnabled: boolean; busy?: boolean }>(), { busy: false })
defineEmits<{ extra: [enabled: boolean] }>()
const { t } = useI18n()
const { reducedMotion } = useMotionPreferences()

const allowance = computed(() => hourlyAllowance(props.usage, props.policy.dailyAllowanceBytes))
const extraRemaining = computed(() => allowance.value.extraRemaining ?? 0n)
const percentage = computed(() => allowance.value.total > 0n ? Number(allowance.value.used * 1000n / allowance.value.total) / 10 : 0)
const usedBytes = computed(() => allowance.value.used.toString())
const totalBytes = computed(() => allowance.value.total.toString())
const remainingBytes = computed(() => allowance.value.remaining.toString())
const extraLeftBytes = computed(() => extraRemaining.value.toString())
const captionKey = computed(() => (allowance.value.extraFunded > 0n ? 'hourly.fullSpeedAllowance' : 'hourly.dailyAllowance'))
const throttleRate = computed(() => props.usage?.reducedRateBytesPerSecond ?? props.policy.reducedRateBytesPerSecond)

// Exact BigInt basis-point widths; the trailing free-remaining segment absorbs
// the sub-basis-point rounding remainder through flex growth.
function segmentWidth(bytes: bigint): string {
  if (allowance.value.total === 0n) return '0%'
  return `${Number((bytes * 10000n) / allowance.value.total) / 100}%`
}

function progressLabel(): string {
  return t('hourly.allowanceUsedOf', { used: formatBytes(usedBytes.value), total: formatBytes(totalBytes.value) })
}

// Thin decorative sine lines over the accent segment base. The paths span two
// 60-unit periods inside a 120-unit viewBox, so translating the 200%-wide group
// by half its width loops seamlessly. Strokes are crisp at any segment size.
const wavePeriod = 60
const waveWidth = 120
const waveSteps = 96
const waves = [
  { amplitude: 4.4, center: 8, phase: 0, strokeWidth: 1.5, opacity: 0.95 },
  { amplitude: 2.7, center: 8.4, phase: Math.PI / 2, strokeWidth: 1.1, opacity: 0.6 },
  { amplitude: 5.6, center: 7.6, phase: Math.PI, strokeWidth: 1, opacity: 0.32 },
].map(wave => {
  const points: string[] = []
  for (let step = 0; step <= waveSteps; step++) {
    const x = (step / waveSteps) * waveWidth
    const y = wave.center - wave.amplitude * Math.sin((2 * Math.PI * x) / wavePeriod + wave.phase)
    points.push(`${step === 0 ? 'M' : 'L'}${x.toFixed(2)} ${y.toFixed(2)}`)
  }
  return { ...wave, path: points.join(' ') }
})

// Consent alone creates no traffic credit. Celebrate a confirmed funded block
// only when it restores a same-day exhausted budget, never an initial load,
// preference toggle or UTC-day reset.
const boosting = shallowRef(false)
let boostTimer: ReturnType<typeof globalThis.setTimeout> | undefined
const graphState = computed(() => ({ exhausted: allowance.value.exhausted, funded: allowance.value.extraFunded, day: props.usage?.dayUTC }))
// Reduced motion swaps to the bar immediately with a static green flash.
const showThrottle = computed(() => allowance.value.exhausted || (boosting.value && !reducedMotion.value))
watch(graphState, (next, previous) => {
  if (!previous.exhausted || next.exhausted || next.funded <= previous.funded || next.day !== previous.day || boosting.value) return
  boosting.value = true
  globalThis.clearTimeout(boostTimer)
  boostTimer = globalThis.setTimeout(() => { boosting.value = false }, reducedMotion.value ? 750 : 1350)
})
onScopeDispose(() => globalThis.clearTimeout(boostTimer))
</script>

<template>
  <figure class="hourly-allowance" :class="{ 'hourly-allowance--reduced': reducedMotion }" data-hourly-allowance>
    <figcaption v-if="!showThrottle">
      <span>{{ t(captionKey) }}</span>
      <strong>{{ formatBytes(totalBytes) }}</strong>
    </figcaption>
    <div v-if="showThrottle" class="hourly-allowance__slot">
      <HourlyThrottleMeter
        :rate-bytes-per-second="throttleRate"
        :extra-available="policy.extraGBPriceTXBMinor !== '0'"
        :extra-price="policy.extraGBPriceTXBMinor"
        :busy="busy"
        :enabled="extraEnabled"
        :boosting="boosting"
        @extra="$emit('extra', $event)"
      />
      <div v-if="boosting" class="hourly-allowance__boost" data-hourly-boost aria-hidden="true">
        <motion.div
          class="hourly-allowance__fly"
          :initial="{ x: '-25%', opacity: 0 }"
          :animate="{ x: ['-25%', '115%'], opacity: [0, 1, 1, 0] }"
          :transition="{ duration: 1.2, ease: 'easeInOut' }"
        >
          <span class="hourly-allowance__trail" />
          <span class="hourly-allowance__sparks" />
          <UIcon class="hourly-allowance__rocket" name="i-ph-rocket-launch-fill" />
        </motion.div>
      </div>
    </div>
    <div v-else class="hourly-allowance__meter">
      <div class="hourly-allowance__bar" role="progressbar" :aria-valuemin="0" :aria-valuemax="100" :aria-valuenow="percentage" :aria-valuetext="progressLabel()" :aria-label="t('hourly.allowanceChart')">
        <span v-if="allowance.freeUsed > 0n" class="hourly-allowance__segment hourly-allowance__segment--free-used" :style="{ width: segmentWidth(allowance.freeUsed) }" />
        <span v-if="allowance.extraUsed > 0n" class="hourly-allowance__segment hourly-allowance__segment--extra-used" :style="{ width: segmentWidth(allowance.extraUsed) }" data-hourly-extra>
          <span class="hourly-allowance__wave" aria-hidden="true">
            <svg viewBox="0 0 120 16" preserveAspectRatio="none" focusable="false">
              <path v-for="(wave, index) in waves" :key="index" :d="wave.path" :stroke-width="wave.strokeWidth" :opacity="wave.opacity" />
            </svg>
          </span>
        </span>
        <span v-if="extraRemaining > 0n" class="hourly-allowance__segment hourly-allowance__segment--extra-left" :style="{ width: segmentWidth(extraRemaining) }" />
        <span v-if="allowance.unknown > 0n" class="hourly-allowance__segment hourly-allowance__segment--unknown" :style="{ width: segmentWidth(allowance.unknown) }" aria-hidden="true" />
        <span class="hourly-allowance__segment hourly-allowance__segment--free-left" />
      </div>
      <dl class="hourly-allowance__legend">
        <div>
          <dt><span class="hourly-allowance__swatch hourly-allowance__swatch--used" aria-hidden="true" />{{ t('dashboard.usedTraffic') }}</dt>
          <dd><motion.span :key="`used-${usedBytes}`" :initial="reducedMotion ? false : { opacity: 0.45 }" :animate="{ opacity: 1 }" :transition="{ duration: reducedMotion ? 0 : 0.2, ease: 'easeOut' }">{{ formatBytes(usedBytes) }}</motion.span></dd>
        </div>
        <div v-if="allowance.freeExhausted">
          <dt><span class="hourly-allowance__swatch hourly-allowance__swatch--extra" aria-hidden="true" />{{ t('hourly.extraLeft') }}</dt>
          <dd><motion.span :key="`extra-${extraLeftBytes}`" :initial="reducedMotion ? false : { opacity: 0.45 }" :animate="{ opacity: 1 }" :transition="{ duration: reducedMotion ? 0 : 0.2, ease: 'easeOut' }">{{ formatBytes(extraLeftBytes) }}</motion.span></dd>
        </div>
        <div v-else>
          <dt><span class="hourly-allowance__swatch hourly-allowance__swatch--remaining" aria-hidden="true" />{{ t('dashboard.remainingTraffic') }}</dt>
          <dd><motion.span :key="`remaining-${remainingBytes}`" :initial="reducedMotion ? false : { opacity: 0.45 }" :animate="{ opacity: 1 }" :transition="{ duration: reducedMotion ? 0 : 0.2, ease: 'easeOut' }">{{ formatBytes(remainingBytes) }}</motion.span></dd>
        </div>
      </dl>
      <div v-if="boosting" class="hourly-allowance__boost hourly-allowance__boost--flash" data-hourly-boost aria-hidden="true">
        <span class="hourly-allowance__flash" />
      </div>
    </div>
  </figure>
</template>

<style scoped>
.hourly-allowance { --hourly-extra: color-mix(in srgb, var(--success) 45%, #00d26a); position: relative; display: grid; gap: 0.4rem; margin: 0; min-width: 0; }
.hourly-allowance figcaption { display: flex; align-items: baseline; justify-content: space-between; gap: 0.75rem; color: var(--text-muted); font-size: 0.75rem; }
.hourly-allowance figcaption strong { color: var(--text); font-family: var(--font-mono); font-size: 0.82rem; }
.hourly-allowance__slot { position: relative; overflow: hidden; border-radius: var(--radius-control); animation: hourly-swap-in 200ms ease-out both; }
.hourly-allowance__meter { position: relative; display: grid; gap: 0.45rem; min-width: 0; animation: hourly-swap-in 200ms ease-out both; }
.hourly-allowance--reduced .hourly-allowance__slot, .hourly-allowance--reduced .hourly-allowance__meter { animation: none; }
@keyframes hourly-swap-in { from { opacity: 0; transform: scale(0.985); } to { opacity: 1; transform: scale(1); } }
.hourly-allowance__bar { display: flex; width: 100%; height: 0.55rem; overflow: hidden; border-radius: 999px; background: var(--line); }
.hourly-allowance__segment { flex: 0 0 auto; height: 100%; min-width: 0; transition: width 320ms var(--ease-out, ease); }
.hourly-allowance--reduced .hourly-allowance__segment { transition: none; }
.hourly-allowance__segment--free-used { background: var(--accent); }
.hourly-allowance__segment--extra-used { position: relative; overflow: hidden; background: var(--accent); }
.hourly-allowance__segment--extra-left { background: linear-gradient(90deg, color-mix(in srgb, var(--accent) 40%, transparent), color-mix(in srgb, var(--hourly-extra) 18%, transparent)); box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--hourly-extra) 24%, transparent); }
.hourly-allowance__segment--unknown { background: repeating-linear-gradient(120deg, var(--text-faint) 0 1px, var(--surface-raised) 1px 4px); }
.hourly-allowance__segment--free-left { flex: 1 1 auto; background: var(--line); }
.hourly-allowance__wave { position: absolute; top: 0; bottom: 0; left: 0; width: 200%; pointer-events: none; animation: hourly-allowance-wave 1.8s linear infinite; }
.hourly-allowance__wave svg { display: block; width: 100%; height: 100%; }
.hourly-allowance__wave path { fill: none; stroke: var(--hourly-extra); stroke-linecap: round; stroke-linejoin: round; vector-effect: non-scaling-stroke; }
.hourly-allowance--reduced .hourly-allowance__wave { animation: none; }
@keyframes hourly-allowance-wave { from { transform: translateX(-50%); } to { transform: translateX(0); } }
.hourly-allowance__legend { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, 10rem), 1fr)); gap: 0.2rem 0.9rem; margin: 0; }
.hourly-allowance__legend div { display: flex; align-items: baseline; gap: 0.4rem; min-width: 0; font-size: 0.72rem; color: var(--text-muted); }
.hourly-allowance__legend dt { display: inline-flex; align-items: center; gap: 0.3rem; min-width: 0; }
.hourly-allowance__legend dd { margin: 0 0 0 auto; color: var(--text); font-family: var(--font-mono); font-weight: 600; white-space: nowrap; }
.hourly-allowance__swatch { width: 0.55rem; height: 0.55rem; flex: 0 0 auto; border-radius: 2px; }
.hourly-allowance__swatch--used { background: var(--accent); }
.hourly-allowance__swatch--extra { background: var(--hourly-extra); }
.hourly-allowance__swatch--remaining { background: var(--line); }
.hourly-allowance__boost { position: absolute; inset: 0; z-index: 2; overflow: hidden; pointer-events: none; }
.hourly-allowance__fly { position: absolute; inset: 0; display: flex; align-items: center; gap: 0.5rem; will-change: transform; }
.hourly-allowance__trail { width: min(8.5rem, 46%); height: 4px; border-radius: 999px; background: linear-gradient(90deg, transparent, color-mix(in srgb, var(--hourly-extra) 80%, white)); box-shadow: 0 0 0.9rem color-mix(in srgb, var(--hourly-extra) 60%, transparent); }
.hourly-allowance__sparks { width: 0.4rem; height: 0.4rem; border-radius: 50%; background: var(--hourly-extra); box-shadow: 1.5rem -0.8rem 0 -0.12rem var(--hourly-extra), 2.7rem 0.9rem 0 -0.14rem var(--hourly-extra), 3.8rem -0.35rem 0 -0.16rem var(--hourly-extra); }
.hourly-allowance__rocket { flex: 0 0 auto; color: var(--hourly-extra); font-size: 2.4rem; filter: drop-shadow(0 0 0.7rem color-mix(in srgb, var(--hourly-extra) 75%, transparent)); }
.hourly-allowance__flash { position: absolute; inset: 0; background: radial-gradient(60% 85% at 50% 50%, color-mix(in srgb, var(--hourly-extra) 34%, transparent), transparent 75%); }
</style>
