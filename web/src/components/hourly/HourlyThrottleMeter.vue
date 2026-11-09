<script setup lang="ts">
import { computed } from 'vue'
import { motion } from 'motion-v'

import { useMotionPreferences } from '@/composables/useMotionPreferences'
import { useI18n } from '@/i18n'
import { formatBytes } from '@/utils/format'

const props = defineProps<{ rateBytesPerSecond: string; extraAvailable: boolean }>()
const { t } = useI18n()
const { reducedMotion } = useMotionPreferences()

const speed = computed(() => t('hourly.rate', { rate: formatBytes(props.rateBytesPerSecond) }))
</script>

<template>
  <motion.div
    class="hourly-throttle"
    data-hourly-throttle
    role="status"
    :initial="reducedMotion ? false : { opacity: 0 }"
    :animate="{ opacity: 1 }"
    :transition="{ duration: reducedMotion ? 0 : 0.3, ease: 'easeOut' }"
  >
    <div class="hourly-throttle__head">
      <span class="hourly-throttle__label">{{ t('hourly.throttledTitle') }}</span>
      <strong class="hourly-throttle__rate">{{ speed }}</strong>
    </div>
    <div class="hourly-throttle__visual" aria-hidden="true">
      <motion.div
        class="hourly-throttle__sag"
        :animate="reducedMotion ? { scaleY: 0.55, opacity: 0.42 } : { scaleY: [1, 0.55, 1], opacity: [0.6, 0.32, 0.6] }"
        :transition="reducedMotion ? { duration: 0 } : { duration: 6.5, repeat: Infinity, ease: 'easeInOut' }"
      >
        <svg viewBox="0 0 120 24" preserveAspectRatio="none" focusable="false">
          <path d="M0 9 C 10 2 18 17 28 10 S 46 4 56 10 S 74 16 84 12 S 104 10 120 14" />
        </svg>
      </motion.div>
      <motion.span
        class="hourly-throttle__flow"
        :animate="reducedMotion ? { x: 0 } : { x: [0, -24] }"
        :transition="reducedMotion ? { duration: 0 } : { duration: 3.6, repeat: Infinity, ease: 'linear' }"
      />
      <motion.span
        class="hourly-throttle__drop"
        :animate="reducedMotion ? { y: 0, opacity: 0.35 } : { y: [0, 5, 0], opacity: [0.5, 0.18, 0.5] }"
        :transition="reducedMotion ? { duration: 0 } : { duration: 5.5, repeat: Infinity, ease: 'easeInOut' }"
      />
    </div>
    <p class="hourly-throttle__hint">{{ extraAvailable ? t('hourly.throttledAdvice') : t('hourly.throttledUnavailable') }}</p>
  </motion.div>
</template>

<style scoped>
.hourly-throttle { --throttle-ink: color-mix(in srgb, #6d8197 60%, var(--text-faint)); display: grid; align-content: space-between; gap: 0.35rem; width: 100%; min-height: 5.4rem; padding: 0.7rem 0.85rem 0.6rem; overflow: hidden; border: 1px solid color-mix(in srgb, #6d8197 40%, var(--line)); border-radius: var(--radius-control); background: linear-gradient(180deg, color-mix(in srgb, #6d8197 9%, var(--surface)), color-mix(in srgb, #435061 14%, var(--surface))); }
.hourly-throttle__head { display: flex; align-items: baseline; justify-content: space-between; gap: 0.75rem; min-width: 0; }
.hourly-throttle__label { color: var(--text-muted); font-size: 0.74rem; }
.hourly-throttle__rate { color: var(--throttle-ink); font: 600 0.9rem var(--font-mono); white-space: nowrap; }
.hourly-throttle__visual { position: relative; height: 2.2rem; margin-inline: -0.85rem; overflow: hidden; }
.hourly-throttle__sag { position: absolute; inset: 0; transform-origin: center; }
.hourly-throttle__sag svg { display: block; width: 100%; height: 100%; }
.hourly-throttle__sag path { fill: none; stroke: color-mix(in srgb, #8296ad 52%, var(--text-faint)); stroke-width: 1.3; vector-effect: non-scaling-stroke; }
.hourly-throttle__flow { position: absolute; left: -24px; right: -24px; top: 50%; height: 2px; margin-top: -1px; background-image: repeating-linear-gradient(90deg, color-mix(in srgb, #8296ad 65%, var(--text-faint)) 0 10px, transparent 10px 24px); opacity: 0.32; }
.hourly-throttle__drop { position: absolute; left: 50%; top: 50%; width: 0.4rem; height: 0.4rem; margin: -0.2rem 0 0 -0.2rem; border-radius: 50%; background: color-mix(in srgb, #8296ad 60%, var(--text-faint)); opacity: 0.4; }
.hourly-throttle__hint { margin: 0; color: var(--text-faint); font-size: 0.74rem; line-height: 1.5; }
</style>
