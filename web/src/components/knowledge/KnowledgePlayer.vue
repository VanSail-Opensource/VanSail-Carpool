<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from '@/i18n'
import DemoPlayer from '@/components/common/DemoPlayer.vue'
import KnowledgeScene from './KnowledgeScene.vue'
import { lessonChapters, lessonIntervals, type KnowledgeLesson } from './knowledgeTopics'
import { knowledgeSources } from './knowledgeSources'

const open = defineModel<boolean>('open', { required: true })
const props = defineProps<{ topic: KnowledgeLesson; returnFocus?: globalThis.HTMLElement | null }>()
const { t } = useI18n()
const pointKeys = ['first', 'second', 'third'] as const
const chapters = computed(() => lessonChapters[props.topic].map(chapter => ({
  icon: chapter.icon,
  label: t(`knowledgeLessons.${props.topic}.steps.${chapter.key}.caption`),
  detail: t(`knowledgeLessons.${props.topic}.steps.${chapter.key}.detail`),
  points: pointKeys.map((key, index) => ({ icon: chapter.pointIcons[index]!, label: t(`knowledgeLessons.${props.topic}.steps.${chapter.key}.points.${key}`) })),
})))
const sources = computed(() => knowledgeSources[props.topic] ?? [])
const sceneStep = (step: number) => lessonChapters[props.topic][step]?.sceneStep ?? 0
// Paired micro-chapters continue one scene instead of restarting its motion.
const scenePhase = (step: number, phase: number) => props.topic === 'subscription' ? phase : (step % 2 + phase) / 2
</script>

<template>
  <DemoPlayer v-model:open="open" :title="t(`knowledge.topics.${topic}.title`)" :description="t(`knowledge.topics.${topic}.subtitle`)" :chapters="chapters" :interval="lessonIntervals[topic]" :reset-key="topic" :return-focus="returnFocus" content-class="knowledge-player">
    <template #default="{ step, phase, reducedMotion }">
      <KnowledgeScene :topic="topic" :step="sceneStep(step)" :phase="scenePhase(step, phase)" :chapter="step" :count="chapters.length" :reduced-motion="reducedMotion" :description="chapters[step]?.detail ?? ''" />
    </template>
    <template #details>
      <nav v-if="sources.length" class="knowledge-player__sources" :aria-label="t('knowledge.sources')">
        <span>{{ t('knowledge.sources') }}</span>
        <a v-for="source in sources" :key="source.url" :href="source.url" target="_blank" rel="noopener noreferrer">{{ t(source.labelKey) }}<UIcon name="i-ph-arrow-up-right" aria-hidden="true" /></a>
      </nav>
    </template>
  </DemoPlayer>
</template>

<style scoped>
.knowledge-player__sources { display: flex; align-items: center; gap: .8rem; min-width: 0; overflow-x: auto; scrollbar-width: thin; color: var(--text-faint); font-size: .65rem; }
.knowledge-player__sources > span { flex: none; }
.knowledge-player__sources a { display: inline-flex; align-items: center; gap: .25rem; flex: none; min-height: 32px; color: var(--text-muted); text-decoration-color: var(--line); text-underline-offset: 3px; }
.knowledge-player__sources a:hover { color: var(--accent); }
.knowledge-player__sources .iconify { flex: none; }
</style>
