<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from '@/i18n'
import DemoPlayer from '@/components/common/DemoPlayer.vue'
import { billingDemoSteps, type BillingDemoMode } from './billingDemo'
import TrafficBillingScene from './TrafficBillingScene.vue'
import HourlyBillingScene from './HourlyBillingScene.vue'
import { guideIntervals } from '@/components/knowledge/knowledgeTopics'

const open = defineModel<boolean>('open', { required: true })
const props = defineProps<{ mode: BillingDemoMode; returnFocus?: globalThis.HTMLElement | null }>()
const { t } = useI18n()
const steps = computed(() => billingDemoSteps[props.mode])
const pointKeys = ['first', 'second', 'third'] as const
const pointIcons = ['i-ph-check-circle-bold', 'i-ph-coins-bold', 'i-ph-info-bold']
const chapters = computed(() => steps.value.map(item => ({
  icon: item.icon, label: t(`payg.demo.${props.mode}.${item.id}`), detail: t(`payg.demo.${props.mode}.${item.id}Detail`),
  points: pointKeys.map((key, index) => ({ icon: pointIcons[index]!, label: t(`payg.demo.${props.mode}.${item.id}Points.${key}`) })),
})))
</script>

<template>
  <DemoPlayer v-model:open="open" :title="t(mode === 'traffic' ? 'payg.title' : 'hourly.title')" :description="t('payg.demo.example')" :chapters="chapters" :interval="guideIntervals[mode]" :reset-key="mode" :return-focus="returnFocus" content-class="billing-demo" :billing-mode="mode">
    <template #default="{ step, phase, reducedMotion }">
      <div class="billing-demo__example"><UIcon name="i-ph-flask-bold" aria-hidden="true" />{{ t('payg.demo.example') }}</div>
      <TrafficBillingScene v-if="mode === 'traffic'" :step="step" :phase="phase" :reduced-motion="reducedMotion" />
      <HourlyBillingScene v-else :step="step" :phase="phase" :reduced-motion="reducedMotion" />
    </template>
  </DemoPlayer>
</template>

<style scoped>
.billing-demo__example { display: flex; align-items: center; gap: .3rem; padding: .7rem .8rem 0; color: var(--text-faint); font-size: .65rem; }
</style>
