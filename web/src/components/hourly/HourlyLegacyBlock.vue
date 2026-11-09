<script setup lang="ts">
import { motion } from 'motion-v'

import { useMotionPreferences } from '@/composables/useMotionPreferences'
import { useI18n } from '@/i18n'

const { t } = useI18n()
const { reducedMotion, offset } = useMotionPreferences()

// Concentric rings expand from the lock like a sealed portal. The last field
// keeps each ring visible when reduced motion removes the pulse.
const rings = [
  { size: '8.5rem', delay: 0, rest: 0.5 },
  { size: '13.5rem', delay: 1.6, rest: 0.32 },
  { size: '19rem', delay: 3.2, rest: 0.2 },
]
</script>

<template>
  <motion.section
    class="hourly-legacy"
    data-hourly-legacy
    :aria-labelledby="'hourly-legacy-title'"
    :initial="reducedMotion ? false : { opacity: 0, y: offset(8), filter: 'blur(6px)' }"
    :animate="{ opacity: 1, y: 0, filter: 'blur(0px)' }"
    :transition="{ duration: reducedMotion ? 0 : 0.32, ease: 'easeOut' }"
  >
    <div class="hourly-legacy__visual" aria-hidden="true">
      <div class="hourly-legacy__aura" />
      <motion.div
        class="hourly-legacy__sweep"
        :animate="reducedMotion ? { rotate: 0 } : { rotate: 360 }"
        :transition="reducedMotion ? { duration: 0 } : { duration: 28, repeat: Infinity, ease: 'linear' }"
      />
      <span v-for="ring in rings" :key="ring.size" class="hourly-legacy__ring" :style="{ width: ring.size, height: ring.size }">
        <motion.span
          class="hourly-legacy__ring-pulse"
          :animate="reducedMotion ? { scale: 1, opacity: ring.rest } : { scale: [0.92, 1.14], opacity: [ring.rest, 0] }"
          :transition="reducedMotion ? { duration: 0 } : { duration: 5.2, repeat: Infinity, delay: ring.delay, ease: 'easeOut' }"
        />
      </span>
      <div class="hourly-legacy__vignette" />
    </div>
    <div class="hourly-legacy__content">
      <span class="hourly-legacy__lock"><UIcon name="i-ph-lock-key" aria-hidden="true" /></span>
      <h3 id="hourly-legacy-title">{{ t('hourly.legacyTitle') }}</h3>
      <p>{{ t('hourly.legacyBlocked') }}</p>
    </div>
  </motion.section>
</template>

<style scoped>
.hourly-legacy { position: relative; isolation: isolate; display: grid; place-items: center; width: 100%; min-height: 12rem; overflow: hidden; padding: clamp(1.25rem, 4vw, 1.9rem); border: 1px solid var(--line); border-radius: var(--radius-control); background: color-mix(in srgb, var(--surface-raised) 82%, transparent); }
.hourly-legacy__visual { position: absolute; inset: 0; z-index: 0; pointer-events: none; }
.hourly-legacy__aura { position: absolute; inset: -25%; background: radial-gradient(62% 72% at 50% 42%, color-mix(in srgb, var(--accent) 24%, transparent), transparent 72%), radial-gradient(46% 58% at 62% 60%, color-mix(in srgb, var(--warning) 16%, transparent), transparent 74%), linear-gradient(160deg, color-mix(in srgb, var(--surface-raised) 92%, transparent), color-mix(in srgb, var(--surface) 88%, transparent)); }
.hourly-legacy__sweep { position: absolute; inset: 0; margin: auto; width: 34rem; height: 34rem; border-radius: 50%; background: conic-gradient(from 0deg, transparent 0deg, color-mix(in srgb, var(--accent) 26%, transparent) 68deg, transparent 148deg, color-mix(in srgb, var(--warning) 20%, transparent) 238deg, transparent 330deg); filter: blur(18px); opacity: 0.8; will-change: transform; }
.hourly-legacy__ring { position: absolute; inset: 0; margin: auto; border-radius: 50%; }
.hourly-legacy__ring-pulse { position: absolute; inset: 0; border-radius: 50%; border: 1px solid color-mix(in srgb, var(--accent) 36%, transparent); }
.hourly-legacy__vignette { position: absolute; inset: 0; background: radial-gradient(80% 90% at 50% 50%, transparent 35%, color-mix(in srgb, var(--surface) 55%, transparent) 100%); }
.hourly-legacy__content { position: relative; z-index: 1; display: grid; justify-items: center; gap: 0.5rem; max-width: 32rem; text-align: center; }
.hourly-legacy__lock { display: inline-flex; align-items: center; justify-content: center; width: 3rem; height: 3rem; border-radius: 50%; color: var(--text); background: color-mix(in srgb, var(--surface) 76%, transparent); border: 1px solid color-mix(in srgb, var(--accent) 34%, var(--line)); box-shadow: 0 0 0 0.35rem color-mix(in srgb, var(--surface) 48%, transparent); font-size: 1.35rem; backdrop-filter: blur(6px); }
.hourly-legacy h3 { margin: 0; font-size: 1.05rem; font-weight: 650; }
.hourly-legacy p { margin: 0; color: var(--text-muted); font-size: 0.83rem; line-height: 1.55; }
</style>
