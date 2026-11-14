<script setup lang="ts">
withDefaults(defineProps<{ path: string; progress: number; active?: boolean; tone?: 'accent' | 'muted' | 'warning' | 'danger' }>(), { tone: 'accent' })
</script>

<template>
  <svg class="demo-flow" :class="`demo-flow--${tone}`" viewBox="0 0 100 100" preserveAspectRatio="none" aria-hidden="true">
    <path :d="path" class="demo-flow__track" />
    <path v-if="active" :d="path" pathLength="100" class="demo-flow__signal" :style="{ strokeDashoffset: -progress * 100 }" />
  </svg>
</template>

<style scoped>
.demo-flow { --flow-color: var(--accent); position: absolute; inset: 0; width: 100%; height: 100%; pointer-events: none; overflow: visible; }
.demo-flow--muted { --flow-color: var(--text-muted); }
.demo-flow--warning { --flow-color: var(--warning, #d4ad72); }
.demo-flow--danger { --flow-color: var(--danger, #e68b96); }
.demo-flow path { fill: none; stroke-width: 1.5; vector-effect: non-scaling-stroke; }
.demo-flow__track { stroke: var(--line); }
.demo-flow__signal { stroke: var(--flow-color); stroke-dasharray: 2 14; stroke-linecap: round; }
</style>
