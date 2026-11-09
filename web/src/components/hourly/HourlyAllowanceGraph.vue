<script setup lang="ts">
import { computed, onScopeDispose, shallowRef, watch } from 'vue'
import { motion } from 'motion-v'

import type { HourlyView } from '@/api/hourly'
import { useMotionPreferences } from '@/composables/useMotionPreferences'
import { useI18n } from '@/i18n'
import { formatBytes } from '@/utils/format'
import HourlyThrottleMeter from './HourlyThrottleMeter.vue'

type Usage = HourlyView['usage']
type Policy = HourlyView['policy']['policy']

const props = defineProps<{ usage: Usage; policy: Policy; extraEnabled: boolean }>()
const { t } = useI18n()
const { reducedMotion } = useMotionPreferences()

function safe(value: string | undefined): bigint {
  return value && /^\d+$/.test(value) ? BigInt(value) : 0n
}

const freeAllowance = computed(() => safe(props.usage?.freeAllowanceBytes ?? props.policy.dailyAllowanceBytes))
const freeUsed = computed(() => safe(props.usage?.freeUsedBytes))
const extraFunded = computed(() => safe(props.usage?.extraFundedBytes))
const extraUsed = computed(() => safe(props.usage?.extraUsedBytes))
const extraReserved = computed(() => safe(props.usage?.extraReservedBytes))
// Purchased extra traffic joins the bar only while automatic purchases are
// enabled; reserved bytes stay out of used and out of the legend remainder.
const total = computed(() => freeAllowance.value + (props.extraEnabled ? extraFunded.value : 0n))
const freeUsedShown = computed(() => (freeUsed.value > freeAllowance.value ? freeAllowance.value : freeUsed.value))
const extraUsedShown = computed(() => (extraUsed.value > extraFunded.value ? extraFunded.value : extraUsed.value))
const extraRemaining = computed(() => (props.extraEnabled ? extraFunded.value - extraUsedShown.value : 0n))
const used = computed(() => freeUsedShown.value + (props.extraEnabled ? extraUsedShown.value : 0n))
const extraLeft = computed(() => {
  const value = extraFunded.value - extraUsed.value - extraReserved.value
  return value > 0n ? value : 0n
})
const remaining = computed(() => total.value - used.value)
const percentage = computed(() => total.value > 0n ? Number(used.value * 1000n / total.value) / 10 : 0)
const usedBytes = computed(() => used.value.toString())
const totalBytes = computed(() => total.value.toString())
const remainingBytes = computed(() => remaining.value.toString())
const extraLeftBytes = computed(() => extraLeft.value.toString())
const captionKey = computed(() => (props.extraEnabled ? 'hourly.fullSpeedAllowance' : 'hourly.dailyAllowance'))
const throttleRate = computed(() => props.usage?.reducedRateBytesPerSecond ?? props.policy.reducedRateBytesPerSecond)
const outOfAllowance = computed(() => freeAllowance.value === 0n ? freeUsed.value > 0n : freeUsed.value >= freeAllowance.value)
const throttled = computed(() => !props.extraEnabled && outOfAllowance.value)

// Exact BigInt basis-point widths; the trailing free-remaining segment absorbs
// the sub-basis-point rounding remainder through flex growth.
function segmentWidth(bytes: bigint): string {
  if (total.value === 0n) return '0%'
  return `${Number((bytes * 10000n) / total.value) / 100}%`
}

function progressLabel(): string {
  return t('hourly.allowanceUsedOf', { used: formatBytes(usedBytes.value), total: formatBytes(totalBytes.value) })
}

// One decorative rocket pass when automatic extra traffic is turned on while
// the reduced-speed state is showing. Initial loads and in-allowance toggles
// never trigger it because the previous snapshot was not exhausted.
const boosting = shallowRef(false)
let boostTimer: ReturnType<typeof globalThis.setTimeout> | undefined
const graphState = computed(() => ({ enabled: props.extraEnabled, out: outOfAllowance.value }))
watch(graphState, (next, previous) => {
  if (!next.enabled || previous.enabled || !previous.out || boosting.value) return
  boosting.value = true
  globalThis.clearTimeout(boostTimer)
  boostTimer = globalThis.setTimeout(() => { boosting.value = false }, reducedMotion.value ? 750 : 1350)
})
onScopeDispose(() => globalThis.clearTimeout(boostTimer))
</script>

<template>
  <figure class="hourly-allowance" :class="{ 'hourly-allowance--reduced': reducedMotion }" data-hourly-allowance>
    <figcaption v-if="!throttled">
      <span>{{ t(captionKey) }}</span>
      <strong>{{ formatBytes(totalBytes) }}</strong>
    </figcaption>
    <HourlyThrottleMeter
      v-if="throttled"
      :rate-bytes-per-second="throttleRate"
      :extra-available="policy.extraGBPriceTXBMinor !== '0'"
    />
    <div v-else class="hourly-allowance__meter">
      <div class="hourly-allowance__bar" role="progressbar" :aria-valuemin="0" :aria-valuemax="100" :aria-valuenow="percentage" :aria-valuetext="progressLabel()" :aria-label="t('hourly.allowanceChart')">
        <span v-if="freeUsedShown > 0n" class="hourly-allowance__segment hourly-allowance__segment--free-used" :style="{ width: segmentWidth(freeUsedShown) }" />
        <span v-if="extraEnabled && extraUsedShown > 0n" class="hourly-allowance__segment hourly-allowance__segment--extra-used" :style="{ width: segmentWidth(extraUsedShown) }" data-hourly-extra>
          <motion.span v-if="!reducedMotion" class="hourly-allowance__wave" :initial="{ x: '-140%' }" :animate="{ x: '160%' }" :transition="{ duration: 2.8, repeat: Infinity, ease: 'linear' }" />
        </span>
        <span v-if="extraEnabled && extraRemaining > 0n" class="hourly-allowance__segment hourly-allowance__segment--extra-left" :style="{ width: segmentWidth(extraRemaining) }" />
        <span class="hourly-allowance__segment hourly-allowance__segment--free-left" />
      </div>
      <dl class="hourly-allowance__legend">
        <div>
          <dt><span class="hourly-allowance__swatch hourly-allowance__swatch--used" aria-hidden="true" />{{ t('dashboard.usedTraffic') }}</dt>
          <dd><motion.span :key="`used-${usedBytes}`" :initial="reducedMotion ? false : { opacity: 0.45 }" :animate="{ opacity: 1 }" :transition="{ duration: reducedMotion ? 0 : 0.2, ease: 'easeOut' }">{{ formatBytes(usedBytes) }}</motion.span></dd>
        </div>
        <div v-if="extraEnabled">
          <dt><span class="hourly-allowance__swatch hourly-allowance__swatch--extra" aria-hidden="true" />{{ t('hourly.extraLeft') }}</dt>
          <dd><motion.span :key="`extra-${extraLeftBytes}`" :initial="reducedMotion ? false : { opacity: 0.45 }" :animate="{ opacity: 1 }" :transition="{ duration: reducedMotion ? 0 : 0.2, ease: 'easeOut' }">{{ formatBytes(extraLeftBytes) }}</motion.span></dd>
        </div>
        <div>
          <dt><span class="hourly-allowance__swatch hourly-allowance__swatch--remaining" aria-hidden="true" />{{ t('dashboard.remainingTraffic') }}</dt>
          <dd><motion.span :key="`remaining-${remainingBytes}`" :initial="reducedMotion ? false : { opacity: 0.45 }" :animate="{ opacity: 1 }" :transition="{ duration: reducedMotion ? 0 : 0.2, ease: 'easeOut' }">{{ formatBytes(remainingBytes) }}</motion.span></dd>
        </div>
      </dl>
    </div>
    <div v-if="boosting" class="hourly-allowance__boost" data-hourly-boost aria-hidden="true">
      <motion.div
        v-if="!reducedMotion"
        class="hourly-allowance__fly"
        :initial="{ x: '-25%', opacity: 0 }"
        :animate="{ x: ['-25%', '105%'], opacity: [0, 1, 1, 0] }"
        :transition="{ duration: 1.2, ease: 'easeInOut' }"
      >
        <span class="hourly-allowance__trail" />
        <span class="hourly-allowance__sparks" />
        <UIcon class="hourly-allowance__rocket" name="i-ph-rocket-launch-fill" />
      </motion.div>
      <span v-else class="hourly-allowance__flash" />
    </div>
  </figure>
</template>

<style scoped>
.hourly-allowance { --hourly-extra: color-mix(in srgb, var(--success) 45%, #00d26a); position: relative; display: grid; gap: 0.4rem; margin: 0; min-width: 0; }
.hourly-allowance figcaption { display: flex; align-items: baseline; justify-content: space-between; gap: 0.75rem; color: var(--text-muted); font-size: 0.75rem; }
.hourly-allowance figcaption strong { color: var(--text); font-family: var(--font-mono); font-size: 0.82rem; }
.hourly-allowance__meter { display: grid; gap: 0.45rem; min-width: 0; }
.hourly-allowance__bar { display: flex; width: 100%; height: 0.55rem; overflow: hidden; border-radius: 999px; background: var(--line); }
.hourly-allowance__segment { flex: 0 0 auto; height: 100%; min-width: 0; transition: width 320ms var(--ease-out, ease); }
.hourly-allowance--reduced .hourly-allowance__segment { transition: none; }
.hourly-allowance__segment--free-used { background: var(--accent); }
.hourly-allowance__segment--extra-used { position: relative; overflow: hidden; background: var(--hourly-extra); box-shadow: 0 0 0.6rem color-mix(in srgb, var(--hourly-extra) 55%, transparent); }
.hourly-allowance__segment--extra-left { background: color-mix(in srgb, var(--hourly-extra) 42%, transparent); }
.hourly-allowance__segment--free-left { flex: 1 1 auto; background: var(--line); }
.hourly-allowance__wave { position: absolute; top: -20%; bottom: -20%; left: 0; width: 65%; background: linear-gradient(90deg, transparent, rgb(255 255 255 / 0.55), transparent); }
.hourly-allowance__legend { display: grid; grid-template-columns: repeat(auto-fit, minmax(7.5rem, 1fr)); gap: 0.2rem 0.9rem; margin: 0; }
.hourly-allowance__legend div { display: flex; align-items: baseline; gap: 0.4rem; min-width: 0; font-size: 0.72rem; color: var(--text-muted); }
.hourly-allowance__legend dt { display: inline-flex; align-items: center; gap: 0.3rem; min-width: 0; }
.hourly-allowance__legend dd { margin: 0 0 0 auto; color: var(--text); font-family: var(--font-mono); font-weight: 600; white-space: nowrap; }
.hourly-allowance__swatch { width: 0.55rem; height: 0.55rem; flex: 0 0 auto; border-radius: 2px; }
.hourly-allowance__swatch--used { background: var(--accent); }
.hourly-allowance__swatch--extra { background: var(--hourly-extra); }
.hourly-allowance__swatch--remaining { background: var(--line); }
.hourly-allowance__boost { position: absolute; inset: 0; z-index: 2; overflow: hidden; pointer-events: none; }
.hourly-allowance__fly { position: absolute; inset: 0; display: flex; align-items: center; gap: 0.35rem; will-change: transform; }
.hourly-allowance__trail { width: min(7rem, 40%); height: 3px; border-radius: 999px; background: linear-gradient(90deg, transparent, color-mix(in srgb, var(--hourly-extra) 80%, white)); box-shadow: 0 0 0.6rem color-mix(in srgb, var(--hourly-extra) 55%, transparent); }
.hourly-allowance__sparks { width: 0.28rem; height: 0.28rem; border-radius: 50%; background: var(--hourly-extra); box-shadow: 0.9rem -0.5rem 0 -0.1rem var(--hourly-extra), 1.6rem 0.55rem 0 -0.12rem var(--hourly-extra), 2.3rem -0.2rem 0 -0.14rem var(--hourly-extra); }
.hourly-allowance__rocket { flex: 0 0 auto; color: var(--hourly-extra); font-size: 1.5rem; filter: drop-shadow(0 0 0.45rem color-mix(in srgb, var(--hourly-extra) 70%, transparent)); }
.hourly-allowance__flash { position: absolute; inset: 0; background: radial-gradient(60% 85% at 50% 50%, color-mix(in srgb, var(--hourly-extra) 34%, transparent), transparent 75%); }
</style>
