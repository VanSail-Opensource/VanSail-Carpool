<script setup lang="ts">
defineProps<{ title: string; icon: string; status?: string; ready?: boolean }>()
</script>

<template>
  <div class="demo-window">
    <div class="demo-window__bar">
      <span class="demo-window__title"><UIcon :name="icon" />{{ title }}</span>
      <span v-if="status" class="demo-window__status" :class="{ 'is-ready': ready }"><i />{{ status }}</span>
      <span v-else class="demo-window__dots"><i /><i /><i /></span>
    </div>
    <div class="demo-window__body"><slot /></div>
    <div v-if="$slots.footer" class="demo-window__footer"><slot name="footer" /></div>
  </div>
</template>

<style scoped>
.demo-window { min-width: 0; overflow: hidden; border: 1px solid var(--line); border-radius: 1.15rem; background: var(--surface-raised, var(--surface)); }
.demo-window__bar { display: flex; align-items: center; justify-content: space-between; gap: .6rem; min-height: 2.5rem; padding: .6rem .85rem; border-bottom: 1px solid var(--line); font-size: .69rem; }
.demo-window__title { display: flex; align-items: center; gap: .4rem; min-width: 0; color: var(--text); font-weight: 600; overflow-wrap: anywhere; }
.demo-window__title .iconify { flex: none; color: var(--accent); font-size: 1rem; }
.demo-window__status { display: flex; flex: none; align-items: center; gap: .3rem; color: var(--text-faint); font-size: .6rem; }
.demo-window__status i, .demo-window__dots i { display: block; width: 5px; height: 5px; border-radius: 50%; background: currentColor; }
.demo-window__status.is-ready { color: var(--success); }
.demo-window__dots { display: flex; gap: 4px; color: var(--text-faint); opacity: .5; }
.demo-window__body { display: grid; gap: .85rem; min-width: 0; padding: 1rem; }
.demo-window__footer { display: flex; flex-wrap: wrap; align-items: center; justify-content: center; gap: .5rem; min-height: 2.4rem; padding: .5rem .85rem; border-top: 1px solid var(--line); color: var(--text-muted); font-size: .66rem; text-align: center; }
@media (max-width: 380px) { .demo-window__body { padding: .75rem; gap: .7rem; } .demo-window__bar { padding-inline: .7rem; } }
</style>
