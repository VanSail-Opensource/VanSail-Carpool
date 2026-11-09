<script setup lang="ts">
import { useI18n } from '@/i18n'

defineEmits<{ addBalance: [] }>()
const { t } = useI18n()
</script>

<template>
  <section class="section-block add-balance" data-add-balance-guide>
    <div class="add-balance__art" aria-hidden="true">
      <span class="add-balance__glow" />
      <span class="add-balance__ring" />
      <span class="add-balance__wallet"><UIcon name="i-ph-wallet-duotone" /></span>
      <span class="add-balance__coin add-balance__coin--one"><UIcon name="i-ph-coin-duotone" /></span>
      <span class="add-balance__coin add-balance__coin--two"><UIcon name="i-ph-coin-duotone" /></span>
    </div>
    <h2>{{ t('dashboard.addBalanceTitle') }}</h2>
    <UButton color="neutral" variant="solid" icon="i-ph-plus-bold" :label="t('billing.addBalance')" data-haptic="open" @click="$emit('addBalance')" />
  </section>
</template>

<style scoped>
.add-balance { display: grid; justify-items: center; gap: .65rem; padding-block: 2rem; overflow: hidden; text-align: center; }
.add-balance h2 { margin: 0; font-size: 1.05rem; font-weight: 650; animation: add-balance-rise 420ms ease-out both; }
.add-balance :deep(button) { animation: add-balance-rise 420ms ease-out 70ms both; }
.add-balance__art { position: relative; display: grid; place-items: center; width: 7rem; height: 5rem; margin-bottom: .1rem; animation: add-balance-rise 420ms ease-out both; }
.add-balance__glow { position: absolute; inset: .15rem; border-radius: 50%; background: radial-gradient(circle, color-mix(in srgb, var(--accent) 28%, transparent), transparent 70%); animation: add-balance-glow 4.8s ease-in-out infinite; }
.add-balance__ring { position: absolute; width: 4.4rem; height: 4.4rem; border: 1px solid color-mix(in srgb, var(--accent) 40%, transparent); border-radius: 50%; animation: add-balance-ring 4.8s ease-out infinite; }
.add-balance__wallet { position: relative; color: var(--accent); font-size: 2.35rem; animation: add-balance-float 4.8s ease-in-out infinite; }
.add-balance__coin { position: absolute; color: var(--warning); font-size: 1.15rem; animation: add-balance-coin 3.2s ease-in-out infinite; }
.add-balance__coin--one { top: .1rem; right: 1.1rem; }
.add-balance__coin--two { bottom: .25rem; left: .95rem; animation-delay: -1.6s; }
.add-balance__art > * { will-change: transform, opacity; }
@keyframes add-balance-rise { from { opacity: 0; transform: translateY(5px); } to { opacity: 1; transform: translateY(0); } }
@keyframes add-balance-float { 0%, 100% { transform: translateY(0); } 50% { transform: translateY(-4px); } }
@keyframes add-balance-glow { 0%, 100% { opacity: .5; transform: scale(.94); } 50% { opacity: .95; transform: scale(1.03); } }
@keyframes add-balance-ring { 0% { opacity: 0; transform: scale(.7); } 35% { opacity: .6; } 80%, 100% { opacity: 0; transform: scale(1.16); } }
@keyframes add-balance-coin { 0%, 100% { opacity: .55; transform: translateY(0) rotate(-6deg); } 50% { opacity: 1; transform: translateY(-6px) rotate(5deg); } }
@media (prefers-reduced-motion: reduce) {
  .add-balance__art, .add-balance h2, .add-balance :deep(button) { animation: none; }
  .add-balance__glow, .add-balance__ring, .add-balance__wallet, .add-balance__coin { animation: none; }
  .add-balance__ring { opacity: .35; }
  .add-balance__art > * { will-change: auto; }
}
</style>
