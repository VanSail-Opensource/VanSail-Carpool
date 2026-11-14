<script setup lang="ts">
import { useI18n } from '@/i18n'
import DemoStage from '@/components/common/DemoStage.vue'
import KnowledgeSubscriptionScene from './KnowledgeSubscriptionScene.vue'
import KnowledgePaymentScene from './KnowledgePaymentScene.vue'
import KnowledgeNetworkScene from './KnowledgeNetworkScene.vue'
import KnowledgeSharingScene from './KnowledgeSharingScene.vue'
import KnowledgeRulesScene from './KnowledgeRulesScene.vue'
import KnowledgeWhyScene from './KnowledgeWhyScene.vue'
import type { KnowledgeLesson } from './knowledgeTopics'

defineProps<{ topic: KnowledgeLesson; step: number; phase: number; chapter: number; count: number; reducedMotion: boolean; description: string }>()
const { t } = useI18n()
const scenes = { subscription: KnowledgeSubscriptionScene, payment: KnowledgePaymentScene, networks: KnowledgeNetworkScene, sharing: KnowledgeSharingScene, rules: KnowledgeRulesScene, why: KnowledgeWhyScene }
</script>

<template>
  <DemoStage class="knowledge-scene" :description="description" :data-knowledge-topic="topic" :data-knowledge-step="step" :data-demo-motion="`${step}:${phase.toFixed(4)}`">
    <div class="knowledge-scene__meta"><span><UIcon name="i-ph-play-circle-bold" />{{ t('knowledge.labels.example') }}</span><span>{{ String(chapter + 1).padStart(2, '0') }} / {{ String(count).padStart(2, '0') }}</span></div>
    <component :is="scenes[topic]" :step="step" :phase="phase" :reduced-motion="reducedMotion" />
  </DemoStage>
</template>

<style scoped>
.knowledge-scene { border: 0; border-radius: 0; background: transparent; }
.knowledge-scene :deep(.demo-stage__canvas) { display: grid; gap: .65rem; aspect-ratio: auto; min-height: 0; padding: .8rem; }
.knowledge-scene__meta { display: flex; align-items: center; justify-content: space-between; gap: .5rem; color: var(--text-faint); font: .6rem var(--font-mono); }
.knowledge-scene__meta > span:first-child { display: flex; align-items: center; gap: .35rem; font-family: var(--font-sans); }
.knowledge-scene__meta .iconify { color: var(--accent); font-size: .9rem; }
@media (max-width: 380px) { .knowledge-scene :deep(.demo-stage__canvas) { padding: .65rem; } }
</style>
