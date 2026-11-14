<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from '@/i18n'
import { between, ease } from '@/components/common/illustrationTimeline'

const props = defineProps<{ step: number; phase: number; matches: boolean; applied: boolean; running: boolean; reducedMotion: boolean }>()
const { t } = useI18n()
const path = computed(() => props.matches ? 'M50 18V46L22 70L50 94' : 'M50 18V46L78 70L50 94')
const packet = computed(() => props.reducedMotion ? .72 : props.phase * 1.5 % 1)
const revealed = (index: number) => props.step > index ? 1 : props.step === index ? ease(between(props.phase, .12, .55)) : 0
const nodes = computed(() => [
  { key: 'entry', icon: 'i-ph-sign-in-bold', x: 50, y: 18, reveal: revealed(0), active: props.applied },
  { key: 'template', icon: 'i-ph-funnel-bold', x: 50, y: 46, reveal: revealed(1), active: props.applied },
  { key: 'exit', icon: 'i-ph-sign-out-bold', x: 22, y: 70, reveal: revealed(2), active: props.applied && props.matches },
  { key: 'default', icon: 'i-ph-arrow-elbow-right-bold', x: 78, y: 70, reveal: revealed(2), active: props.applied && !props.matches },
  { key: 'destination', icon: 'i-ph-globe-bold', x: 50, y: 94, reveal: revealed(2), active: props.applied },
])
</script>

<template>
  <div class="routing-flow" :data-demo-selected-exit="matches ? 'exit' : 'default'">
    <svg viewBox="0 0 100 112" preserveAspectRatio="none" aria-hidden="true">
      <path d="M50 18V46L22 70L50 94M50 46L78 70L50 94" class="routing-flow__tracks" />
      <path v-if="applied" :d="path" class="routing-flow__route" />
      <path v-if="applied && running" :d="path" class="routing-flow__packet" pathLength="1" stroke-dasharray=".04 .96" :stroke-dashoffset="-packet" />
    </svg>
    <span v-for="node in nodes" :key="node.key" class="routing-flow__node" :class="{ 'is-active': node.active }" :style="{ left: `${node.x}%`, top: `${node.y / 112 * 100}%`, opacity: .25 + .75 * node.reveal, transform: `translate(-50%, -50%) scale(${.88 + .12 * node.reveal})` }"><UIcon :name="node.icon" /><span>{{ t(`diy.demo.nodes.${node.key}`) }}</span></span>
    <span class="routing-flow__decision" :class="{ 'is-active': applied }"><UIcon :name="matches ? 'i-ph-check-bold' : 'i-ph-minus-bold'" />{{ t(`diy.demo.${matches ? 'match' : 'noMatch'}`) }}</span>
  </div>
</template>

<style scoped>
.routing-flow { position: relative; height: 238px; min-width: 0; }
.routing-flow > svg { position: absolute; inset: 0; width: 100%; height: 100%; overflow: visible; }
.routing-flow__tracks, .routing-flow__route, .routing-flow__packet { fill: none; vector-effect: non-scaling-stroke; stroke-linecap: round; stroke-linejoin: round; }
.routing-flow__tracks { stroke: var(--line); stroke-width: 1.5; }
.routing-flow__route { stroke: color-mix(in srgb, var(--accent) 45%, var(--line)); stroke-width: 2; }
.routing-flow__packet { stroke: var(--accent); stroke-width: 3; }
.routing-flow__node { position: absolute; display: flex; align-items: center; gap: .35rem; justify-content: center; min-width: 5.6rem; max-width: 42%; padding: .5rem .65rem; transform: translate(-50%, -50%); border: 1px solid var(--line); border-radius: var(--radius-control); background: var(--surface-raised); color: var(--text-faint); font-size: .7rem; line-height: 1.25; text-align: center; }
.routing-flow__node > .iconify { flex: none; font-size: 1rem; }
.routing-flow__node.is-active { color: var(--accent); border-color: color-mix(in srgb, var(--accent) 40%, var(--line)); }
.routing-flow__decision { position: absolute; top: 39%; left: 72%; display: flex; align-items: center; justify-content: center; gap: .25rem; max-width: 28%; padding: .3rem .4rem; color: var(--text-faint); font-size: .6rem; text-align: center; }
.routing-flow__decision > .iconify { flex: none; }
.routing-flow__decision.is-active { color: var(--accent); }
@media (max-width: 380px) { .routing-flow__node { min-width: 4.5rem; gap: .25rem; padding: .5rem; font-size: .65rem; } }
</style>
