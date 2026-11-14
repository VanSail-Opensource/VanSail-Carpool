<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from '@/i18n'
import DemoStage from '@/components/common/DemoStage.vue'
import DemoNode from '@/components/common/DemoNode.vue'
import DemoFlow from '@/components/common/DemoFlow.vue'
import DemoPointer from '@/components/common/DemoPointer.vue'
import { interactionCue } from '@/components/common/illustrationTimeline'

const props = defineProps<{ step: number; phase: number; reducedMotion: boolean; description: string }>()
const { t } = useI18n()
const allowed = [
  { key: 'browsing', icon: 'i-ph-browser-bold' }, { key: 'streaming', icon: 'i-ph-monitor-play-bold' },
  { key: 'ai', icon: 'i-ph-sparkle-bold' }, { key: 'work', icon: 'i-ph-briefcase-bold' },
]
const blocked = [
  { key: 'crawler', icon: 'i-ph-bug-bold' }, { key: 'registration', icon: 'i-ph-robot-bold' },
  { key: 'sharing', icon: 'i-ph-share-network-bold' }, { key: 'torrent', icon: 'i-ph-download-bold' },
  { key: 'spam', icon: 'i-ph-envelope-simple-bold' }, { key: 'attacks', icon: 'i-ph-warning-octagon-bold' },
]
const good = computed(() => allowed[Math.min(3, Math.floor(props.phase * 4))]!)
const bad = computed(() => blocked[props.step === 2 ? 3 + Math.min(2, Math.floor(props.phase * 3)) : Math.min(2, Math.floor(props.phase * 3))]!)
const cue = computed(() => interactionCue(props.phase, props.step === 3 ? [{ at: .35, x: 50, y: 28 }, { at: .7, x: 50, y: 72 }] : []))
const flow = computed(() => props.reducedMotion ? .5 : props.phase * 3)
</script>

<template>
  <DemoStage :description="description" class="knowledge-rules-scene" :data-knowledge-step="step">
    <DemoFlow path="M 16 28 H 84" :progress="flow" active />
    <DemoFlow path="M 16 72 H 50" :progress="flow" :active="step > 0" tone="danger" />
    <DemoNode :icon="good.icon" :label="t(`knowledge.labels.${good.key}`)" :x="16" :y="28" active />
    <DemoNode :icon="step === 3 ? 'i-ph-funnel-bold' : 'i-ph-shield-check-bold'" :label="t('knowledge.labels.protocol')" :x="50" :y="28" active />
    <DemoNode icon="i-ph-check-circle-bold" :label="t('knowledge.labels.allowed')" :x="84" :y="28" active />
    <DemoNode :icon="bad.icon" :label="t(`knowledge.labels.${bad.key}`)" :x="16" :y="72" :active="step > 0" tone="danger" />
    <DemoNode :icon="step === 3 ? 'i-ph-gauge-bold' : 'i-ph-prohibit-bold'" :label="t(step === 3 ? 'knowledge.labels.connections' : 'knowledge.labels.blocked')" :x="50" :y="72" :active="step > 0" tone="danger" />
    <span class="knowledge-rules-scene__stop" :class="{ 'is-visible': step > 0 }"><UIcon name="i-ph-hand-palm-bold" /></span>
    <DemoPointer :cue="cue" :reduced-motion="reducedMotion" />
  </DemoStage>
</template>

<style scoped>
.knowledge-rules-scene__stop { position: absolute; left: 84%; top: 70%; transform: translate(-50%, -50%) scale(.8); opacity: .15; color: var(--danger); font-size: 2.2rem; transition: opacity .25s, transform .25s; }
.knowledge-rules-scene__stop.is-visible { opacity: 1; transform: translate(-50%, -50%) scale(1); }
@media (prefers-reduced-motion: reduce) { .knowledge-rules-scene__stop { transition: none; } }
</style>
