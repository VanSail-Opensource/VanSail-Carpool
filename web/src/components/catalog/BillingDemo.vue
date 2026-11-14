<script setup lang="ts">
import { computed } from 'vue'
import { useIllustrationPlayback } from '@/composables/useIllustrationPlayback'
import { useI18n } from '@/i18n'
import DemoTransport from '@/components/common/DemoTransport.vue'
import { billingDemoSteps, type BillingDemoMode } from './billingDemo'
import TrafficBillingScene from './TrafficBillingScene.vue'
import HourlyBillingScene from './HourlyBillingScene.vue'

const open = defineModel<boolean>('open', { required: true })
const props = defineProps<{ mode: BillingDemoMode; returnFocus?: globalThis.HTMLElement | null }>()
const { t } = useI18n()
const steps = computed(() => billingDemoSteps[props.mode])
const playback = useIllustrationPlayback({ active: () => open.value, stepCount: () => steps.value.length, resetKey: () => props.mode, interval: 4400 })
const { step, phase, progress, elapsed, duration, playing, reducedMotion } = playback
const current = computed(() => steps.value[step.value] ?? steps.value[0])
const captionKey = computed(() => `payg.demo.${props.mode}.${current.value.id}`)
const chapters = computed(() => steps.value.map(item => ({ icon: item.icon, label: t(`payg.demo.${props.mode}.${item.id}`) })))
function restoreFocus(event: globalThis.Event): void {
  event.preventDefault()
  if (props.returnFocus?.isConnected) props.returnFocus.focus({ preventScroll: true })
}
</script>

<template>
  <UModal v-model:open="open" :title="t(mode === 'traffic' ? 'payg.title' : 'hourly.title')" :description="t('payg.demo.example')" :content="{ onCloseAutoFocus: restoreFocus }" :transition="!reducedMotion" :close="{ 'aria-label': t('common.close') }" :ui="{ content: 'sm:max-w-xl', body: 'p-4 sm:p-6' }">
    <template #body>
      <div class="billing-demo" :data-billing-mode="mode" :data-demo-playing="playing">
        <TrafficBillingScene v-if="mode === 'traffic'" :step="step" :phase="phase" :reduced-motion="reducedMotion" />
        <HourlyBillingScene v-else :step="step" :phase="phase" :reduced-motion="reducedMotion" />
        <div class="billing-demo__caption"><UIcon :name="current.icon" aria-hidden="true" /><span>{{ t(captionKey) }}</span></div>
        <p class="sr-only" role="status" aria-live="polite" aria-atomic="true">{{ t(`${captionKey}Detail`) }}</p>
        <DemoTransport :chapters="chapters" :step="step" :progress="progress" :elapsed="elapsed" :duration="duration" :playing="playing" :reduced-motion="reducedMotion" @select="playback.selectStep" @seek="playback.seek" @toggle="playback.toggle" @replay="playback.replay" />
      </div>
    </template>
  </UModal>
</template>

<style scoped>
.billing-demo { display: grid; gap: .8rem; min-width: 0; }
.billing-demo__caption { display: flex; align-items: center; justify-content: center; gap: .5rem; min-height: 2.8rem; color: var(--text); font-size: .83rem; text-align: center; }
.billing-demo__caption .iconify { flex: none; color: var(--accent); font-size: 1.2rem; }
</style>
