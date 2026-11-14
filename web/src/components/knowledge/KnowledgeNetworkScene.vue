<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from '@/i18n'
import DemoStage from '@/components/common/DemoStage.vue'
import DemoNode from '@/components/common/DemoNode.vue'
import DemoFlow from '@/components/common/DemoFlow.vue'
import DemoPointer from '@/components/common/DemoPointer.vue'
import { between, interactionCue } from '@/components/common/illustrationTimeline'

const props = defineProps<{ step: number; phase: number; reducedMotion: boolean; description: string }>()
const { t } = useI18n()
const progress = computed(() => props.reducedMotion ? .5 : (props.step + props.phase) * 1.6)
const entered = computed(() => props.step > 0 || props.phase >= .24)
const broadband = computed(() => props.step === 2 || (props.step === 3 && props.phase >= .48))
const international = computed(() => props.step === 1 || (props.step === 3 && !broadband.value))
const cue = computed(() => interactionCue(props.phase, props.step === 0
  ? [{ at: .24, x: 36, y: 44 }]
  : props.step === 1 ? [{ at: .24, x: 65, y: 23 }]
    : props.step === 2 ? [{ at: .24, x: 65, y: 66 }]
      : [{ at: .18, x: 65, y: 23 }, { at: .48, x: 65, y: 66 }]))
const badge = computed(() => ['carriers', 'globalReach', 'localIsp', 'exit'][props.step] ?? 'exit')
</script>

<template>
  <DemoStage :description="description" class="network-scene" :data-knowledge-step="step">
    <div class="network-scene__badge"><UIcon name="i-ph-path" />{{ t(`knowledge.labels.${badge}`) }}</div>
    <DemoFlow path="M 12 46 H 36" :progress="progress" :active="entered" />
    <DemoFlow path="M 36 46 Q 43 24 65 24 T 89 46" :progress="progress" :active="international" />
    <DemoFlow path="M 36 46 Q 43 68 65 68 T 89 46" :progress="progress" :active="broadband" />
    <DemoNode icon="i-ph-device-mobile" :label="t('knowledge.labels.you')" :x="12" :y="50" :active="entered" />
    <DemoNode icon="i-ph-lightning" :label="t('knowledge.labels.optimized')" :x="36" :y="50" :active="entered" />
    <DemoNode icon="i-ph-globe-hemisphere-west" :label="t('knowledge.labels.international')" :x="65" :y="28" :active="international" />
    <DemoNode icon="i-ph-house-line" :label="t('knowledge.labels.broadband')" :x="65" :y="72" :active="broadband" />
    <DemoNode icon="i-ph-browser" :label="t('knowledge.labels.service')" :x="89" :y="50" :active="international || broadband" />
    <div class="network-scene__distinction" :style="{ opacity: step === 3 ? 1 : .35 }">
      <span><UIcon name="i-ph-gauge" />{{ t('knowledge.labels.route') }}</span>
      <UIcon name="i-ph-not-equals" />
      <span><UIcon name="i-ph-fingerprint" />{{ t('knowledge.labels.reputation') }}</span>
    </div>
    <span class="network-scene__signal" :style="{ opacity: step === 0 ? between(phase, .3, .5) : 0, transform: `translateY(${step === 0 && !reducedMotion ? -between(phase, .3, .9) * 8 : 0}px)` }"><UIcon name="i-ph-cell-signal-full" /></span>
    <DemoPointer :cue="cue" :reduced-motion="reducedMotion" />
  </DemoStage>
</template>

<style scoped>
.network-scene__badge { position: absolute; top: 5%; left: 5%; display: flex; align-items: center; gap: .4rem; color: var(--text-muted); font-size: .7rem; }
.network-scene__badge .iconify { color: var(--accent); font-size: 1rem; }
.network-scene__distinction { position: absolute; inset-inline: 4%; bottom: 5%; display: flex; align-items: center; justify-content: center; gap: .6rem; color: var(--text-muted); font-size: .66rem; transition: opacity .3s; }
.network-scene__distinction > span { display: inline-flex; align-items: center; gap: .3rem; }
.network-scene__distinction .iconify { flex: none; font-size: 1rem; }
.network-scene__signal { position: absolute; left: 33%; top: 24%; color: var(--accent); font-size: 1.4rem; }
@media (prefers-reduced-motion: reduce) { .network-scene__distinction { transition: none; } }
</style>
