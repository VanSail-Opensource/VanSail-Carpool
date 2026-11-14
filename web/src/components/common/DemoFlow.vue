<script setup lang="ts">
import { computed } from 'vue'
import { between, ease } from './illustrationTimeline'

const props = defineProps<{
  nodes: readonly { icon: string; label: string }[]
  phase: number
  active?: number
  blocked?: boolean
  reducedMotion: boolean
}>()
const travel = computed(() => props.reducedMotion ? 1 : ease(between(props.phase, .08, .86)))
const x = (index: number) => (index + .5) * 100 / props.nodes.length
</script>

<template>
  <div class="demo-flow" :class="{ 'is-blocked': blocked }" :style="{ '--flow-count': nodes.length }">
    <svg class="demo-flow__paths" viewBox="0 0 100 42" preserveAspectRatio="none">
      <path v-for="(_, index) in nodes.slice(1)" :key="index" :d="`M ${x(index)} 21 H ${x(index + 1)}`" class="demo-flow__line" />
      <path :d="`M ${x(0)} 21 H ${x(nodes.length - 1)}`" class="demo-flow__trace" pathLength="1" :stroke-dasharray="`${travel} 1`" />
      <circle v-if="!reducedMotion && !blocked" :cx="x(0) + (x(nodes.length - 1) - x(0)) * travel" cy="21" r="1.1" class="demo-flow__packet" />
    </svg>
    <div v-for="(node, index) in nodes" :key="index" class="demo-flow__node" :class="{ 'is-active': active === index, 'is-passed': travel >= index / Math.max(1, nodes.length - 1) }">
      <span class="demo-flow__icon"><UIcon :name="blocked && index === nodes.length - 1 ? 'i-ph-prohibit-bold' : node.icon" /></span>
      <span class="demo-flow__label">{{ node.label }}</span>
    </div>
  </div>
</template>

<style scoped>
.demo-flow { position: relative; display: grid; grid-template-columns: repeat(var(--flow-count), minmax(0, 1fr)); align-items: start; gap: .4rem; min-width: 0; padding-block: .5rem; }
.demo-flow__paths { position: absolute; top: .5rem; left: 0; width: 100%; height: 42px; overflow: visible; }
.demo-flow__line { fill: none; stroke: var(--line); stroke-width: .7; }
.demo-flow__trace { fill: none; stroke: var(--accent); stroke-width: .6; }
.demo-flow__packet { fill: var(--accent); }
.demo-flow__node { position: relative; display: grid; justify-items: center; gap: .55rem; min-width: 0; color: var(--text-faint); }
.demo-flow__icon { display: grid; place-items: center; width: 42px; height: 42px; border: 1px solid var(--line); border-radius: .9rem; background: var(--surface-raised, var(--surface)); font-size: 1.25rem; }
.demo-flow__label { max-width: 100%; text-align: center; font-size: .65rem; line-height: 1.4; overflow-wrap: anywhere; }
.is-passed .demo-flow__icon { color: var(--accent); border-color: var(--accent-line); }
.is-active .demo-flow__icon { color: var(--accent); background: var(--accent-soft); }
.is-active .demo-flow__label { color: var(--text); }
.is-blocked .demo-flow__trace { stroke: var(--warning, var(--text-muted)); }
.is-blocked .demo-flow__node:last-child .demo-flow__icon { color: var(--warning, var(--text-muted)); border-color: currentColor; }
@media (max-width: 380px) { .demo-flow { gap: .2rem; } .demo-flow__label { font-size: .6rem; } }
</style>
