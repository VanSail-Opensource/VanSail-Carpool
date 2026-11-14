<script setup lang="ts">
import { defineAsyncComponent } from 'vue'
import { useI18n } from '@/i18n'
import KnowledgeCard from './KnowledgeCard.vue'
import { isKnowledgeLesson, knowledgeTopics } from './knowledgeTopics'
import { loadKnowledgePlayer, preloadKnowledgePlayer } from './knowledgePlayers'
import { useKnowledgePlayer } from './useKnowledgePlayer'

const KnowledgePlayer = defineAsyncComponent(() => loadKnowledgePlayer('subscription'))
const BillingDemo = defineAsyncComponent(() => loadKnowledgePlayer('traffic'))
const RoutingDemo = defineAsyncComponent(() => loadKnowledgePlayer('routing'))
const { t } = useI18n()
const { selected, open, trigger, loadingTopic, errorTopic, play } = useKnowledgePlayer()
</script>

<template>
  <div class="page page--knowledgebase">
    <header class="page-header knowledge-page__header"><div><p class="eyebrow">{{ t('dashboard.aroundTx') }}</p><h1>{{ t('knowledge.title') }}</h1><p>{{ t('knowledge.subtitle') }}</p></div><span class="knowledge-page__count"><UIcon name="i-ph-play-circle-bold" aria-hidden="true" />{{ t('knowledge.topicCount', { count: knowledgeTopics.length }) }}</span></header>
    <div class="knowledge-page__grid"><KnowledgeCard v-for="(topic, index) in knowledgeTopics" :key="topic.id" :topic="topic" :index="index" :loading="loadingTopic === topic.id" :error="errorTopic === topic.id" @intent="preloadKnowledgePlayer(topic.id)" @play="play(topic.id, $event)" /></div>
    <KnowledgePlayer v-if="open && isKnowledgeLesson(selected)" v-model:open="open" :topic="selected" :return-focus="trigger" />
    <BillingDemo v-else-if="open && (selected === 'traffic' || selected === 'hourly')" v-model:open="open" :mode="selected" :return-focus="trigger" />
    <RoutingDemo v-else-if="open && selected === 'routing'" v-model:open="open" :return-focus="trigger" />
  </div>
</template>

<style scoped>
.page--knowledgebase { display: grid; gap: 1.5rem; max-width: 1240px; }
.knowledge-page__header { display: flex; justify-content: space-between; align-items: end; gap: 1rem; margin: 0; }
.knowledge-page__header .eyebrow { margin: 0 0 .5rem; color: var(--accent); }
.knowledge-page__header h1 { margin: 0; }
.knowledge-page__header p:last-child { margin: .5rem 0 0; }
.knowledge-page__count { display: flex; align-items: center; gap: .4rem; flex: none; font: .7rem var(--font-mono); color: var(--text-faint); }
.knowledge-page__grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, 260px), 1fr)); gap: 1rem; }
@media (max-width: 600px) { .knowledge-page__header { align-items: start; flex-direction: column; gap: .7rem; } }
</style>
