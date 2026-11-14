<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from '@/i18n'
import DemoPlayer from '@/components/common/DemoPlayer.vue'
import KnowledgeNetworkScene from './KnowledgeNetworkScene.vue'
import KnowledgePaymentScene from './KnowledgePaymentScene.vue'
import KnowledgeSharingScene from './KnowledgeSharingScene.vue'
import KnowledgeRulesScene from './KnowledgeRulesScene.vue'
import KnowledgeWhyScene from './KnowledgeWhyScene.vue'
import { chapterKeys, lessonIcons, type KnowledgeLesson } from './knowledgeTopics'
import { knowledgeSources } from './knowledgeSources'

const open = defineModel<boolean>('open', { required: true })
const props = defineProps<{ topic: KnowledgeLesson; returnFocus?: globalThis.HTMLElement | null }>()
const { t } = useI18n()
const scenes = { networks: KnowledgeNetworkScene, payment: KnowledgePaymentScene, sharing: KnowledgeSharingScene, rules: KnowledgeRulesScene, why: KnowledgeWhyScene }
const chapters = computed(() => chapterKeys.map((key, index) => ({
  icon: lessonIcons[props.topic][index]!,
  label: t(`knowledgeLessons.${props.topic}.steps.${key}.caption`),
  detail: t(`knowledgeLessons.${props.topic}.steps.${key}.detail`),
})))
const sources = computed(() => knowledgeSources[props.topic] ?? [])
</script>

<template>
  <DemoPlayer v-model:open="open" :title="t(`knowledge.topics.${topic}.title`)" :description="t(`knowledge.topics.${topic}.subtitle`)" :chapters="chapters" :interval="6500" :reset-key="topic" :return-focus="returnFocus" content-class="knowledge-player">
    <template #default="{ step, phase, reducedMotion }"><component :is="scenes[topic]" :step="step" :phase="phase" :reduced-motion="reducedMotion" :description="chapters[step]?.detail ?? ''" /></template>
    <template #details>
      <details class="knowledge-player__details"><summary>{{ t('knowledge.details') }}</summary><ol><li v-for="chapter in chapters" :key="chapter.label"><strong>{{ chapter.label }}</strong><p>{{ chapter.detail }}</p></li></ol><div v-if="sources.length" class="knowledge-player__sources"><strong>{{ t('knowledge.sources') }}</strong><a v-for="source in sources" :key="source.url" :href="source.url" target="_blank" rel="noopener noreferrer">{{ t(source.labelKey) }}<UIcon name="i-ph-arrow-up-right" aria-hidden="true" /></a></div></details>
    </template>
  </DemoPlayer>
</template>

<style scoped>
.knowledge-player__details { color: var(--text-muted); font-size: .76rem; }
.knowledge-player__details summary { width: fit-content; min-height: 44px; display: list-item; align-content: center; cursor: pointer; color: var(--text-faint); }
.knowledge-player__details summary:focus-visible { outline: 2px solid var(--accent); outline-offset: 3px; }
.knowledge-player__details ol { display: grid; gap: .9rem; padding-left: 1.15rem; margin: .4rem 0 1rem; }
.knowledge-player__details strong { color: var(--text); font-weight: 600; }
.knowledge-player__details p { margin: .2rem 0 0; line-height: 1.6; }
.knowledge-player__sources { display: grid; gap: .4rem; border-top: 1px solid var(--line); padding-top: .75rem; }
.knowledge-player__sources a { display: flex; align-items: center; gap: .3rem; min-height: 32px; color: var(--accent); overflow-wrap: anywhere; }
.knowledge-player__sources .iconify { flex: none; }
</style>
