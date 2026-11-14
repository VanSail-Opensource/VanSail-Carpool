<script setup lang="ts">
import { computed } from 'vue'
import { useIllustrationPlayback } from '@/composables/useIllustrationPlayback'
import { useI18n } from '@/i18n'
import { billingDemoSteps, type BillingDemoMode } from './billingDemo'
import TrafficBillingScene from './TrafficBillingScene.vue'
import HourlyBillingScene from './HourlyBillingScene.vue'

const open = defineModel<boolean>('open', { required: true })
const props = defineProps<{ mode: BillingDemoMode; returnFocus?: globalThis.HTMLElement | null }>()
const { t } = useI18n()
const steps = computed(() => billingDemoSteps[props.mode])
const { step, playing, reducedMotion, selectStep, toggle, replay } = useIllustrationPlayback({
  active: () => open.value,
  stepCount: () => steps.value.length,
  resetKey: () => props.mode,
  interval: 3200,
})
const current = computed(() => steps.value[step.value] ?? steps.value[0])
const captionKey = computed(() => `payg.demo.${props.mode}.${current.value.id}`)
function restoreFocus(event: globalThis.Event): void {
  event.preventDefault()
  if (props.returnFocus?.isConnected) props.returnFocus.focus({ preventScroll: true })
}
</script>

<template>
  <UModal v-model:open="open" :title="t(mode === 'traffic' ? 'payg.title' : 'hourly.title')" :description="t('payg.demo.example')" :content="{ onCloseAutoFocus: restoreFocus }" :close="{ 'aria-label': t('common.close') }" :ui="{ content: 'sm:max-w-xl', body: 'p-4 sm:p-6', footer: 'justify-between' }">
    <template #body>
      <div class="billing-demo" :class="{ 'billing-demo--playing': playing && !reducedMotion }" :data-billing-mode="mode">
        <TrafficBillingScene v-if="mode === 'traffic'" :step="step" :playing="playing && !reducedMotion" />
        <HourlyBillingScene v-else :step="step" :playing="playing && !reducedMotion" />
        <div class="billing-demo__caption">
          <UIcon :name="current.icon" aria-hidden="true" />
          <span>{{ t(captionKey) }}</span>
        </div>
        <p class="sr-only" role="status" aria-live="polite" aria-atomic="true">{{ t(`${captionKey}Detail`) }}</p>
        <div class="billing-demo__steps" role="group" :aria-label="t('payg.demo.steps')">
          <UButton v-for="(item, index) in steps" :key="item.id" square color="neutral" :variant="index === step ? 'solid' : 'ghost'" :icon="item.icon" :aria-label="t(`payg.demo.${mode}.${item.id}`)" :title="t(`payg.demo.${mode}.${item.id}`)" :aria-pressed="index === step" :class="{ 'billing-demo__step--done': index < step }" :data-demo-step="index" @click="selectStep(index)" />
        </div>
      </div>
    </template>
    <template #footer>
      <span class="billing-demo__position">{{ t('payg.demo.position', { step: step + 1, total: steps.length }) }}</span>
      <div class="billing-demo__controls">
        <UButton square color="neutral" variant="ghost" icon="i-ph-skip-back" :aria-label="t('payg.demo.previous')" :title="t('payg.demo.previous')" :disabled="step === 0" @click="selectStep(step - 1)" />
        <UButton v-if="!reducedMotion" square color="neutral" variant="soft" :icon="playing ? 'i-ph-pause' : 'i-ph-play'" :aria-label="t(playing ? 'payg.demo.pause' : 'payg.demo.play')" :title="t(playing ? 'payg.demo.pause' : 'payg.demo.play')" @click="toggle" />
        <UButton square color="neutral" variant="ghost" icon="i-ph-skip-forward" :aria-label="t('payg.demo.next')" :title="t('payg.demo.next')" :disabled="step === steps.length - 1" @click="selectStep(step + 1)" />
        <UButton square color="neutral" variant="ghost" icon="i-ph-arrow-counter-clockwise" :aria-label="t('payg.demo.replay')" :title="t('payg.demo.replay')" @click="replay" />
      </div>
    </template>
  </UModal>
</template>

<style scoped>
.billing-demo { display: grid; gap: 1rem; min-width: 0; }
.billing-demo__caption { display: flex; align-items: center; justify-content: center; gap: .5rem; min-height: 2.5rem; color: var(--text); font-size: .85rem; text-align: center; }
.billing-demo__caption .iconify { flex: none; color: var(--accent); font-size: 1.2rem; }
.billing-demo__steps { display: flex; justify-content: safe center; gap: .25rem; min-width: 0; overflow-x: auto; padding: .25rem; border-top: 1px solid var(--line); }
.billing-demo__steps :deep(button) { flex: none; min-width: 44px; min-height: 44px; border-radius: 50%; }
.billing-demo__step--done { color: var(--accent); }
.billing-demo__controls { display: flex; gap: .15rem; }
.billing-demo__controls :deep(button) { min-width: 44px; min-height: 44px; }
.billing-demo__position { color: var(--text-faint); font: .75rem var(--font-mono); }
</style>
