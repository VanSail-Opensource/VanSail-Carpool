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
const traffic = computed(() => props.step > 0 || props.phase >= .5)
const flow = computed(() => props.reducedMotion ? .5 : props.phase * 3)
const cue = computed(() => interactionCue(props.phase, props.step === 0 ? [{ at: .2, x: 18, y: 23 }, { at: .55, x: 82, y: 23 }] : props.step === 3 ? [{ at: .55, x: 82, y: 77 }] : []))
const support = computed(() => props.step === 3 && props.phase >= .5)
</script>

<template>
  <DemoStage :description="description" class="knowledge-why-scene" :data-knowledge-step="step">
    <DemoFlow :path="traffic ? 'M 82 23 L 50 50' : 'M 18 23 L 50 50'" :progress="flow" active />
    <DemoFlow path="M 50 50 L 18 77" :progress="flow" :active="step >= 2" />
    <DemoFlow path="M 50 50 L 82 77" :progress="flow" :active="step >= 3" />
    <DemoNode icon="i-ph-clock-bold" :label="t('knowledge.labels.hourly')" :x="18" :y="23" :active="!traffic || step > 0" />
    <DemoNode icon="i-ph-arrows-down-up-bold" :label="t('knowledge.labels.traffic')" :x="82" :y="23" :active="traffic" />
    <DemoNode icon="i-ph-code-bold" :label="t('knowledge.labels.code')" :x="50" :y="50" :active="step >= 1" />
    <DemoNode icon="i-ph-heartbeat-bold" :label="t('knowledge.labels.quality')" :x="18" :y="77" :active="step >= 2" />
    <DemoNode :icon="support ? 'i-ph-chats-circle-bold' : 'i-ph-lock-key-bold'" :label="t(support ? 'knowledge.labels.support' : 'knowledge.labels.privacy')" :x="82" :y="77" :active="step >= 3" />
    <span v-if="step === 1" class="knowledge-why-scene__build" :style="{ transform: `scaleX(${Math.min(1, phase * 1.5)})` }" />
    <DemoPointer :cue="cue" :reduced-motion="reducedMotion" />
  </DemoStage>
</template>

<style scoped>
.knowledge-why-scene__build { position: absolute; width: 20%; height: 3px; background: var(--accent); top: 66%; left: 40%; transform-origin: left; border-radius: 2px; }
</style>
