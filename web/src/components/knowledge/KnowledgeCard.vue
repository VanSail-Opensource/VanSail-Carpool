<script setup lang="ts">
import { useI18n } from '@/i18n'
import type { KnowledgeTopic } from './knowledgeTopics'

defineProps<{ topic: KnowledgeTopic; index: number; loading?: boolean; error?: boolean }>()
const emit = defineEmits<{ play: [event: globalThis.MouseEvent]; intent: [] }>()
const { t } = useI18n()
const points = ['first', 'second', 'third'] as const
function reload(): void { globalThis.location.reload() }
</script>

<template>
  <article class="knowledge-card" :class="{ 'knowledge-card--start': topic.id === 'subscription' }" :data-knowledge-topic="topic.id">
    <header class="knowledge-card__header"><span class="knowledge-card__icon"><UIcon :name="topic.icon" aria-hidden="true" /></span><span v-if="topic.id === 'subscription'" class="knowledge-card__start">{{ t('knowledge.labels.startHere') }}</span><span v-else class="knowledge-card__index">{{ String(index + 1).padStart(2, '0') }}</span></header>
    <div class="knowledge-card__heading"><h2>{{ t(`knowledge.topics.${topic.id}.title`) }}</h2><p>{{ t(`knowledge.topics.${topic.id}.subtitle`) }}</p></div>
    <ul class="knowledge-card__points"><li v-for="(point, i) in points" :key="point"><UIcon :name="topic.points[i]" aria-hidden="true" /><span>{{ t(`knowledge.topics.${topic.id}.points.${point}`) }}</span></li></ul>
    <footer class="knowledge-card__footer"><span>{{ t('knowledge.duration', { seconds: topic.seconds }) }}</span><UButton color="primary" variant="soft" icon="i-ph-play-fill" :loading="loading" :aria-busy="loading || undefined" :label="t(error ? 'common.tryAgain' : 'knowledge.watch')" :aria-label="`${t(error ? 'common.tryAgain' : 'knowledge.watch')} · ${t(`knowledge.topics.${topic.id}.title`)}`" @pointerenter="emit('intent')" @focus="emit('intent')" @click="emit('play', $event)" /></footer>
    <p class="sr-only" role="status" aria-live="polite">{{ loading ? t('knowledge.loadingGuide', { title: t(`knowledge.topics.${topic.id}.title`) }) : '' }}</p>
    <div v-if="error" class="knowledge-card__error"><p role="alert">{{ t('knowledge.loadFailed') }}</p><UButton color="neutral" variant="ghost" :label="t('recovery.reload')" @click="reload" /></div>
  </article>
</template>

<style scoped>
.knowledge-card { position: relative; display: grid; grid-template-rows: auto auto 1fr auto; gap: 1rem; min-width: 0; padding: 1.2rem; border: 1px solid var(--line); border-radius: var(--radius-panel); background: var(--surface); transition: border-color .2s, transform .2s; overflow: hidden; }
.knowledge-card::before { content: ''; position: absolute; inset: 0 auto 0 0; width: 2px; background: var(--accent); opacity: .25; transition: opacity .2s; }
.knowledge-card__header, .knowledge-card__footer { display: flex; align-items: center; justify-content: space-between; gap: .75rem; }
.knowledge-card__icon { display: grid; place-items: center; width: 2.5rem; height: 2.5rem; color: var(--accent); background: var(--accent-soft); border-radius: var(--radius-control); font-size: 1.35rem; }
.knowledge-card__index { color: var(--text-faint); font: .75rem var(--font-mono); }
.knowledge-card--start { border-color: var(--accent-line); }
.knowledge-card--start::before { width: 3px; opacity: 1; }
.knowledge-card__start { color: var(--accent); font-size: .67rem; font-weight: 650; }
.knowledge-card__heading h2 { margin: 0; font-size: 1.08rem; line-height: 1.35; letter-spacing: -.025em; }
.knowledge-card__heading p { margin: .35rem 0 0; color: var(--text-faint); font-size: .76rem; }
.knowledge-card__points { display: grid; align-content: start; gap: .65rem; list-style: none; padding: 0; margin: 0; }
.knowledge-card__points li { display: flex; align-items: start; gap: .5rem; color: var(--text-muted); font-size: .79rem; line-height: 1.45; }
.knowledge-card__points .iconify { flex: none; color: var(--accent); font-size: 1rem; margin-top: .05rem; }
.knowledge-card__footer { border-top: 1px solid var(--line); padding-top: .75rem; }
.knowledge-card__footer > span { color: var(--text-faint); font: .7rem var(--font-mono); }
.knowledge-card__footer :deep(button) { min-height: 44px; }
.knowledge-card__error { display: grid; gap: .3rem; justify-items: start; color: var(--text-muted); font-size: .75rem; line-height: 1.5; }
.knowledge-card__error p { margin: 0; }
.knowledge-card__error :deep(button) { min-height: 44px; }
.knowledge-card:focus-within { border-color: var(--accent-line); }
@media (hover: hover) and (prefers-reduced-motion: no-preference) { .knowledge-card:hover { transform: translateY(-2px); border-color: var(--accent-line); } .knowledge-card:hover::before { opacity: 1; } }
</style>
