<script setup lang="ts">
defineProps<{ description: string }>()
</script>

<template>
  <div class="demo-stage" role="img" :aria-label="description">
    <div class="demo-stage__canvas" aria-hidden="true"><slot /></div>
  </div>
</template>

<style scoped>
.demo-stage { position: relative; min-width: 0; overflow: hidden; border: 1px solid var(--line); border-radius: 1rem; background: var(--surface-soft, var(--surface)); }
.demo-stage__canvas { position: relative; width: 100%; aspect-ratio: 1.65; min-height: 240px; isolation: isolate; }
</style>
