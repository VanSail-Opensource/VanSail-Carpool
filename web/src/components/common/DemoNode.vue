<script setup lang="ts">
withDefaults(defineProps<{ icon: string; label: string; x: number; y: number; active?: boolean; tone?: 'accent' | 'muted' | 'warning' | 'danger'; detail?: string }>(), { tone: 'accent', detail: '' })
</script>

<template>
  <div class="demo-node" :class="[{ 'is-active': active }, `demo-node--${tone}`]" :style="{ left: `${x}%`, top: `${y}%` }">
    <span class="demo-node__icon"><UIcon :name="icon" /></span>
    <span class="demo-node__label">{{ label }}</span>
    <small v-if="detail" class="demo-node__detail">{{ detail }}</small>
  </div>
</template>

<style scoped>
.demo-node { --node-color: var(--accent); position: absolute; width: 26%; display: grid; justify-items: center; gap: .35rem; transform: translate(-50%, -50%); color: var(--text-faint); text-align: center; z-index: 2; }
.demo-node--muted { --node-color: var(--text-muted); }
.demo-node--warning { --node-color: var(--warning, #d4ad72); }
.demo-node--danger { --node-color: var(--danger, #e68b96); }
.demo-node__icon { display: grid; place-items: center; width: clamp(2.5rem, 7vw, 3.3rem); height: clamp(2.5rem, 7vw, 3.3rem); border: 1px solid var(--line); border-radius: .9rem; background: var(--surface-raised, var(--surface)); font-size: 1.45rem; transition: transform .25s, color .25s, border-color .25s; }
.demo-node.is-active { color: var(--text); }
.demo-node.is-active .demo-node__icon { color: var(--node-color); border-color: color-mix(in srgb, var(--node-color) 58%, transparent); transform: scale(1.06); }
.demo-node__label { max-width: 100%; font-size: clamp(.62rem, 1.9vw, .76rem); font-weight: 650; line-height: 1.25; overflow-wrap: anywhere; }
.demo-node__detail { max-width: 100%; color: var(--text-muted); font: .62rem var(--font-mono); overflow-wrap: anywhere; }
@media (prefers-reduced-motion: reduce) { .demo-node__icon { transition: none; } }
</style>
