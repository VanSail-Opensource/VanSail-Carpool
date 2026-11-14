<script setup lang="ts">
defineProps<{ cue: { x: number; y: number; press: number; opacity: number }; reducedMotion?: boolean }>()
</script>

<template>
  <div v-if="!reducedMotion" class="demo-pointer" aria-hidden="true" :style="{ transform: `translate(calc(${cue.x}% - var(--pointer-inset, 0px)), ${cue.y}%)`, opacity: cue.opacity, '--press': cue.press }">
    <span class="demo-pointer__ripple" />
    <UIcon name="i-ph-cursor-fill" class="demo-pointer__cursor" />
    <span class="demo-pointer__touch" />
  </div>
</template>

<style scoped>
.demo-pointer { position: absolute; inset: 0; pointer-events: none; z-index: 4; }
.demo-pointer__cursor { display: block; color: var(--text); font-size: 1.35rem; filter: drop-shadow(0 1px 2px var(--canvas)); transform: scale(calc(1 - var(--press) * .2)) rotate(-12deg); transform-origin: 0 0; }
.demo-pointer__ripple { position: absolute; width: 2.2rem; height: 2.2rem; border: 1px solid var(--accent); border-radius: 50%; opacity: calc(var(--press) * .8); transform: translate(-50%, -50%) scale(calc(.4 + var(--press) * .7)); }
.demo-pointer__touch { display: none; width: 1.15rem; height: 1.15rem; border: 2px solid var(--text); border-radius: 50%; background: color-mix(in srgb, var(--accent) 22%, transparent); transform: translate(-50%, -50%) scale(calc(1 - var(--press) * .3)); }
@media (hover: none), (pointer: coarse), (max-width: 600px) { .demo-pointer__cursor { display: none; } .demo-pointer__touch { display: block; } }
@media (prefers-reduced-motion: reduce) { .demo-pointer { display: none; } }
</style>
