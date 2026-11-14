<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from '@/i18n'
import DemoStage from '@/components/common/DemoStage.vue'
import DemoNode from '@/components/common/DemoNode.vue'
import DemoFlow from '@/components/common/DemoFlow.vue'
import DemoPointer from '@/components/common/DemoPointer.vue'
import { between, ease, interactionCue } from '@/components/common/illustrationTimeline'

const props = defineProps<{ step: number; phase: number; reducedMotion: boolean; description: string }>()
const { t } = useI18n()
const progress = computed(() => props.reducedMotion ? .5 : (props.step + props.phase) * 1.2)
const limited = computed(() => props.step === 0 ? 0 : props.step === 1 ? ease(between(props.phase, .08, .5)) : 1)
const users = computed(() => Array.from({ length: 12 }, (_, index) => {
  const from = { x: 8 + index % 3 * 8, y: 29 + Math.floor(index / 3) * 10 }
  const to = { x: 12 + index % 2 * 10, y: 38 + Math.floor(index / 2) * 12 }
  return { id: index, x: from.x + (to.x - from.x) * limited.value, y: from.y + (to.y - from.y) * limited.value, opacity: index < 4 ? 1 : 1 - limited.value }
}))
const guarded = computed(() => props.step > 2 || (props.step === 2 && props.phase >= .3))
const checks = [
  { key: 'reputation', icon: 'i-ph-fingerprint', at: .2 },
  { key: 'device', icon: 'i-ph-device-mobile', at: .4 },
  { key: 'behavior', icon: 'i-ph-cursor-click', at: .6 },
] as const
const cue = computed(() => interactionCue(props.phase, props.step === 1
  ? [{ at: .3, x: 50, y: 43 }]
  : props.step === 2 ? [{ at: .3, x: 50, y: 20 }]
    : props.step === 3 ? [{ at: .2, x: 24, y: 82 }, { at: .4, x: 50, y: 82 }, { at: .6, x: 76, y: 82 }] : []))
</script>

<template>
  <DemoStage :description="description" class="sharing-scene" :data-knowledge-step="step">
    <span class="sharing-scene__eyebrow">{{ t(`knowledge.labels.${step === 0 ? 'crowd' : 'members'}`) }}</span>
    <DemoFlow path="M 27 46 H 50 H 83" :progress="progress" active />
    <div v-for="user in users" :key="user.id" class="sharing-scene__user" :style="{ transform: `translate(${user.x}%, ${user.y}%)`, opacity: user.opacity }"><UIcon name="i-ph-user-fill" /></div>
    <span class="sharing-scene__member-label">{{ t(`knowledge.labels.${step === 0 ? 'crowd' : 'members'}`) }}</span>
    <DemoNode :icon="step === 0 ? 'i-ph-network' : 'i-ph-hard-drives'" :label="t(`knowledge.labels.${step === 0 ? 'publicIp' : 'dedicatedIp'}`)" :x="50" :y="50" active />
    <DemoNode icon="i-ph-browser" :label="t('knowledge.labels.service')" :x="83" :y="50" active />
    <div class="sharing-scene__guard" :style="{ opacity: step >= 2 ? 1 : .15 }">
      <DemoFlow path="M 29 19 Q 48 17 50 43" :progress="progress" :active="step === 2 && !guarded" tone="danger" />
      <span class="sharing-scene__bot" :style="{ opacity: guarded ? .3 : 1 }"><UIcon name="i-ph-robot" /></span>
      <span class="sharing-scene__shield" :class="{ 'is-guarded': guarded }"><UIcon :name="guarded ? 'i-ph-shield-check-fill' : 'i-ph-shield'" /></span>
      <span class="sharing-scene__guard-label">{{ t('knowledge.labels.controlled') }}</span>
    </div>
    <div class="sharing-scene__checks" :style="{ opacity: step === 3 ? 1 : .25 }">
      <span v-for="check in checks" :key="check.key" :class="{ 'is-checked': step === 3 && phase >= check.at }"><UIcon :name="check.icon" /><small>{{ t(`knowledge.labels.${check.key}`) }}</small></span>
    </div>
    <DemoPointer :cue="cue" :reduced-motion="reducedMotion" />
  </DemoStage>
</template>

<style scoped>
.sharing-scene__eyebrow { position: absolute; top: 6%; left: 5%; color: var(--text-muted); font-size: .7rem; }
.sharing-scene__user { position: absolute; inset: 0; pointer-events: none; color: var(--accent); font-size: 1.15rem; }
.sharing-scene__member-label { position: absolute; top: 67%; left: 4%; width: 27%; color: var(--text-muted); text-align: center; font-size: .65rem; }
.sharing-scene__guard { position: absolute; inset: 0; transition: opacity .3s; }
.sharing-scene__bot { position: absolute; top: 16%; left: 27%; color: var(--danger, #e68b96); font-size: 1.5rem; }
.sharing-scene__shield { position: absolute; top: 16%; left: 47%; color: var(--text-faint); font-size: 1.65rem; }
.sharing-scene__shield.is-guarded { color: var(--accent); }
.sharing-scene__guard-label { position: absolute; top: 20%; left: 60%; font-size: .65rem; color: var(--text-muted); }
.sharing-scene__checks { position: absolute; inset: auto 10% 6%; display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: .6rem; transition: opacity .3s; }
.sharing-scene__checks > span { display: grid; gap: .25rem; justify-items: center; color: var(--text-faint); text-align: center; }
.sharing-scene__checks > span > .iconify { font-size: 1.25rem; }
.sharing-scene__checks > span > small { font-size: .62rem; line-height: 1.2; }
.sharing-scene__checks > .is-checked { color: var(--accent); }
@media (prefers-reduced-motion: reduce) { .sharing-scene__guard, .sharing-scene__checks { transition: none; } }
</style>
