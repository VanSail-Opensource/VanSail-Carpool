<script setup lang="ts">
import { useI18n } from '@/i18n'
import { preloadMemberView } from '@/router/memberViews'
import { preloadKnowledgePlayer } from './knowledgePlayers'

const { t } = useI18n()
const icons = ['i-ph-device-mobile-bold', 'i-ph-path-bold', 'i-ph-globe-hemisphere-west-bold']
function warm(): void {
  void preloadMemberView('/knowledgebase')
  void preloadKnowledgePlayer('networks')
}
</script>

<template>
  <section class="knowledge-home" aria-labelledby="knowledge-home-title">
    <div class="knowledge-home__copy"><p class="knowledge-home__eyebrow"><UIcon name="i-ph-play-circle-fill" aria-hidden="true" />{{ t('knowledge.homeEyebrow') }}</p><h2 id="knowledge-home-title">{{ t('knowledge.homeTitle') }}</h2><p>{{ t('knowledge.homeNote') }}</p><UButton to="/knowledgebase" icon="i-ph-play-fill" color="primary" variant="soft" :label="t('knowledge.start')" data-haptic="navigate" @pointerenter="warm" @focus="warm" /></div>
    <div class="knowledge-home__visual" aria-hidden="true"><span v-for="(icon, index) in icons" :key="icon" :style="{ '--order': index }"><UIcon :name="icon" /></span><i /></div>
  </section>
</template>

<style scoped>
.knowledge-home { display: grid; grid-template-columns: 1fr auto; align-items: center; gap: 1.25rem; padding: clamp(1.2rem, 3vw, 2rem); border: 1px solid var(--accent-line); border-radius: var(--radius-panel); background: var(--surface); overflow: hidden; }
.knowledge-home__copy { min-width: 0; }
.knowledge-home__eyebrow { display: flex; align-items: center; gap: .4rem; margin: 0 0 .65rem; color: var(--accent); font-size: .7rem; font-weight: 700; letter-spacing: .06em; }
.knowledge-home__copy h2 { margin: 0; font-size: clamp(1.25rem, 3vw, 1.7rem); letter-spacing: -.035em; line-height: 1.25; }
.knowledge-home__copy > p:not(.knowledge-home__eyebrow) { margin: .55rem 0 1rem; color: var(--text-muted); font-size: .82rem; max-width: 34rem; }
.knowledge-home__copy :deep(a) { min-height: 44px; }
.knowledge-home__visual { position: relative; display: flex; align-items: center; gap: 1.6rem; padding: .75rem; }
.knowledge-home__visual::before { content: ''; position: absolute; left: .75rem; right: .75rem; height: 1px; background: var(--accent-line); }
.knowledge-home__visual > span { z-index: 1; display: grid; place-items: center; width: 3.4rem; height: 3.4rem; background: var(--surface-raised); border: 1px solid var(--accent-line); border-radius: .9rem; color: var(--accent); font-size: 1.7rem; }
.knowledge-home__visual > i { position: absolute; left: .75rem; width: .35rem; height: .35rem; border-radius: 50%; background: var(--accent); }
@media (prefers-reduced-motion: no-preference) { .knowledge-home__visual > span { animation: knowledge-hello .7s both; animation-delay: calc(var(--order) * .15s); } .knowledge-home__visual > i { animation: knowledge-packet 3s ease-in-out 1; } }
@keyframes knowledge-hello { from { opacity: .4; transform: translateY(8px); } to { opacity: 1; transform: none; } }
@keyframes knowledge-packet { from { transform: translateX(0); } to { transform: translateX(10rem); } }
@media (max-width: 680px) { .knowledge-home { grid-template-columns: minmax(0, 1fr); gap: .5rem; } .knowledge-home__visual { grid-row: 1; justify-self: start; gap: 1.5rem; padding: .25rem 0 .7rem; } .knowledge-home__visual > span { width: 2.25rem; height: 2.25rem; font-size: 1.2rem; } }
</style>
