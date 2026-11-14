<script setup lang="ts">
import { computed, shallowRef, watch } from 'vue'
import { useI18n } from '@/i18n'
import DemoPlayer from '@/components/common/DemoPlayer.vue'
import RoutingDemoScene from './RoutingDemoScene.vue'
import { guideIntervals } from '@/components/knowledge/knowledgeTopics'

const open = defineModel<boolean>('open', { required: true })
defineProps<{ returnFocus?: globalThis.HTMLElement | null }>()
const { t } = useI18n()
const steps = [
  { key: 'entry', icon: 'i-ph-sign-in-bold', points: ['i-ph-key-bold', 'i-ph-sign-in-bold', 'i-ph-devices-bold'] },
  { key: 'template', icon: 'i-ph-funnel-bold', points: ['i-ph-funnel-bold', 'i-ph-globe-bold', 'i-ph-list-numbers-bold'] },
  { key: 'exit', icon: 'i-ph-sign-out-bold', points: ['i-ph-arrows-left-right-bold', 'i-ph-key-bold', 'i-ph-sign-out-bold'] },
  { key: 'check', icon: 'i-ph-shield-check', points: ['i-ph-key-bold', 'i-ph-funnel-bold', 'i-ph-arrows-clockwise-bold'] },
  { key: 'save', icon: 'i-ph-floppy-disk', points: ['i-ph-floppy-disk-bold', 'i-ph-queue-bold', 'i-ph-clock-countdown-bold'] },
  { key: 'confirm', icon: 'i-ph-check-circle-bold', points: ['i-ph-hard-drives-bold', 'i-ph-check-circle-bold', 'i-ph-path-bold'] },
  { key: 'route', icon: 'i-ph-path', points: ['i-ph-list-numbers-bold', 'i-ph-sign-out-bold', 'i-ph-arrow-elbow-right-bold'] },
  { key: 'refresh', icon: 'i-ph-arrows-clockwise-bold', points: ['i-ph-floppy-disk-bold', 'i-ph-arrows-clockwise-bold', 'i-ph-plugs-connected-bold'] },
] as const
const overrideMatch = shallowRef<boolean | null>(null)
watch(open, () => { overrideMatch.value = null })
const matches = (step: number, phase: number) => overrideMatch.value ?? (step < 6 || (step === 6 && phase < .55))
const pointKeys = ['first', 'second', 'third'] as const
const chapters = computed(() => steps.map(item => ({
  icon: item.icon, label: t(`diy.demo.labels.${item.key}`), caption: t(`diy.demo.captions.${item.key}`), detail: t(`diy.demo.descriptions.${item.key}`),
  points: pointKeys.map((key, index) => ({ icon: item.points[index]!, label: t(`diy.demo.points.${item.key}.${key}`) })),
})))
const scenarios = computed(() => [
  { value: 'match', label: t('diy.demo.match'), icon: 'i-ph-check-circle' },
  { value: 'default', label: t('diy.demo.noMatch'), icon: 'i-ph-arrow-elbow-right' },
])
function scenario(value: string | number, selectStep: (step: number) => void) { overrideMatch.value = value === 'match'; selectStep(6) }
</script>

<template>
  <DemoPlayer v-model:open="open" :title="t('diy.demo.title')" :description="t('diy.demo.description')" :chapters="chapters" :interval="guideIntervals.routing" :return-focus="returnFocus" content-class="routing-demo" @replay="overrideMatch = null">
    <template #default="{ step, phase, reducedMotion }">
      <div class="routing-demo__example"><UIcon name="i-ph-flask" aria-hidden="true" />{{ t('diy.demo.example') }}</div>
      <RoutingDemoScene :step="step" :phase="phase" :matches="matches(step, phase)" :reduced-motion="reducedMotion" :description="chapters[step]?.detail ?? ''" />
    </template>
    <template #controls="{ step, phase, selectStep }">
      <UTabs class="routing-demo__scenarios" :items="scenarios" :model-value="matches(step, phase) ? 'match' : 'default'" :content="false" size="sm" :aria-label="t('diy.demo.destination')" :ui="{ list: 'w-full', trigger: 'flex-1 min-h-10' }" @update:model-value="scenario($event, selectStep)" />
    </template>
  </DemoPlayer>
</template>

<style scoped>
.routing-demo__example { display: flex; align-items: center; gap: .35rem; padding: .7rem .85rem 0; color: var(--text-faint); font-size: .7rem; }
.routing-demo__scenarios { min-width: 0; }
</style>
