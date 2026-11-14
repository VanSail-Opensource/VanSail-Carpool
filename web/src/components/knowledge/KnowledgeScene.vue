<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from '@/i18n'
import DemoStage from '@/components/common/DemoStage.vue'
import DemoThreeScene from '@/components/common/DemoThreeScene.vue'
import DemoPointer from '@/components/common/DemoPointer.vue'
import { demoThreeActive } from '@/components/common/demoThreeModel'
import { interactionCue } from '@/components/common/illustrationTimeline'
import KnowledgeSubscriptionAction from './KnowledgeSubscriptionAction.vue'
import { sceneLabels } from './knowledgeSceneLabels'
import type { KnowledgeLesson } from './knowledgeTopics'

const props = defineProps<{ topic: KnowledgeLesson; step: number; phase: number; chapter: number; count: number; reducedMotion: boolean; description: string }>()
const { t } = useI18n()
const label = computed(() => sceneLabels[props.topic][props.step] ?? sceneLabels[props.topic][0]!)
const cue = computed(() => interactionCue(props.phase, [{ at: .25, x: 50, y: 48 }]))
const exitIcon = computed(() => demoThreeActive({ topic: 'networks', step: props.step, phase: props.phase, reducedMotion: props.reducedMotion }).includes('broadband') ? 'i-ph-house-line-bold' : 'i-ph-globe-hemisphere-west-bold')
</script>

<template>
  <DemoStage class="knowledge-scene" :description="description" :data-knowledge-step="step">
    <div class="knowledge-scene__visual">
      <DemoThreeScene :topic="topic" :step="step" :phase="phase" :reduced-motion="reducedMotion" />
      <div class="knowledge-scene__heading"><span><UIcon :name="label[0]" />{{ t(`knowledge.labels.${label[1]}`) }}</span><small v-if="topic === 'payment'">{{ t('knowledge.labels.example') }}</small><small v-else>{{ String(chapter + 1).padStart(2, '0') }} / {{ String(count).padStart(2, '0') }}</small></div>
      <DemoPointer v-if="topic !== 'subscription'" :cue="cue" :reduced-motion="reducedMotion" />
    </div>
    <KnowledgeSubscriptionAction v-if="topic === 'subscription'" :step="step" :phase="phase" :reduced-motion="reducedMotion" />
    <div v-else-if="topic === 'payment'" class="knowledge-payment-equation">
      <span v-if="step === 2"><small>{{ t('knowledge.labels.withdrawal') }}</small><strong>{{ '12.545678' }}</strong></span>
      <UIcon v-if="step === 2" name="i-ph-minus-bold" />
      <span v-if="step === 2"><small>{{ t('knowledge.labels.fee') }}</small><strong>{{ '0.200000' }}</strong></span>
      <UIcon v-if="step === 2" name="i-ph-equals-bold" />
      <span><small>{{ t('knowledge.labels.recipient') }}</small><strong>{{ '12.345678' }} <small>{{ 'USDT' }}</small></strong></span>
    </div>
    <div v-else-if="topic === 'networks'" class="knowledge-scene__legend"><span><UIcon name="i-ph-lightning-bold" />{{ t('knowledge.labels.entry') }}</span><UIcon name="i-ph-arrow-right-bold" /><span><UIcon :name="exitIcon" />{{ t('knowledge.labels.exit') }}</span><UIcon name="i-ph-arrow-right-bold" /><span><UIcon name="i-ph-browser-bold" />{{ t('knowledge.labels.service') }}</span></div>
  </DemoStage>
</template>

<style scoped>
.knowledge-scene { border: 0; border-radius: 0; background: transparent; }
.knowledge-scene :deep(.demo-stage__canvas) { aspect-ratio: auto; min-height: 0; }
.knowledge-scene__visual { position: relative; min-width: 0; }
.knowledge-scene__heading { position: absolute; inset: .8rem .9rem auto; display: flex; align-items: center; justify-content: space-between; gap: .5rem; color: var(--text-muted); font-size: .7rem; }
.knowledge-scene__heading > span { display: flex; align-items: center; gap: .35rem; }
.knowledge-scene__heading .iconify { color: var(--accent); font-size: 1rem; }
.knowledge-scene__heading > small { color: var(--text-faint); font: .65rem var(--font-mono); }
.knowledge-scene__legend { display: flex; align-items: center; justify-content: center; gap: .55rem; min-width: 0; padding: .5rem .75rem; border-top: 1px solid var(--line); color: var(--text-faint); font-size: .65rem; }
.knowledge-scene__legend span { display: inline-flex; align-items: center; gap: .3rem; color: var(--text-muted); }
.knowledge-payment-equation { display: flex; align-items: center; justify-content: center; gap: .5rem; min-width: 0; padding: .55rem .75rem; border-top: 1px solid var(--line); background: var(--surface-soft, var(--surface)); color: var(--accent); }
.knowledge-payment-equation > span { display: grid; gap: .2rem; text-align: center; }
.knowledge-payment-equation small { font-size: .58rem; font-weight: 400; color: var(--text-faint); }
.knowledge-payment-equation strong { color: var(--text); font: 500 clamp(.6rem, 2vw, .9rem) var(--font-mono); }
.knowledge-payment-equation > .iconify { flex: none; font-size: .8rem; }
@media (max-width: 420px) { .knowledge-scene__heading { inset: .6rem .7rem auto; font-size: .65rem; } .knowledge-payment-equation { gap: .25rem; } }
</style>
