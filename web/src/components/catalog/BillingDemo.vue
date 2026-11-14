<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from '@/i18n'
import DemoPlayer from '@/components/common/DemoPlayer.vue'
import { billingDemoSteps, type BillingDemoMode } from './billingDemo'
import TrafficBillingScene from './TrafficBillingScene.vue'
import HourlyBillingScene from './HourlyBillingScene.vue'

const open = defineModel<boolean>('open', { required: true })
const props = defineProps<{ mode: BillingDemoMode; returnFocus?: globalThis.HTMLElement | null }>()
const { t } = useI18n()
const steps = computed(() => billingDemoSteps[props.mode])
const chapters = computed(() => steps.value.map(item => ({ icon: item.icon, label: t(`payg.demo.${props.mode}.${item.id}`), detail: t(`payg.demo.${props.mode}.${item.id}Detail`) })))
</script>

<template>
  <DemoPlayer v-model:open="open" :title="t(mode === 'traffic' ? 'payg.title' : 'hourly.title')" :description="t('payg.demo.example')" :chapters="chapters" :interval="4400" :reset-key="mode" :return-focus="returnFocus" content-class="billing-demo" :billing-mode="mode">
    <template #default="{ step, phase, reducedMotion }">
      <TrafficBillingScene v-if="mode === 'traffic'" :step="step" :phase="phase" :reduced-motion="reducedMotion" />
      <HourlyBillingScene v-else :step="step" :phase="phase" :reduced-motion="reducedMotion" />
    </template>
  </DemoPlayer>
</template>
