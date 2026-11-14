<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from '@/i18n'
import { formatBytes } from '@/utils/format'

const props = defineProps<{ usageMiB: number; blocks: number; expired: boolean; phase: number; moving: boolean; reducedMotion: boolean }>()
const { t } = useI18n()
const upload = computed(() => Math.round(props.usageMiB / 4))
const download = computed(() => props.usageMiB - upload.value)
const packet = computed(() => props.reducedMotion ? .5 : props.phase * 2 % 1)
const used = (block: number) => Math.min(1, Math.max(0, (props.usageMiB - (block - 1) * 1024) / 1024))
const bytes = (mib: number) => formatBytes(String(mib * 1024 * 1024))
</script>

<template>
  <div class="traffic-graph" data-demo-graph="traffic">
    <div class="traffic-graph__directions">
      <span><UIcon name="i-ph-upload-simple-bold" /><small>{{ t('payg.demo.upload') }}</small><strong>{{ bytes(upload) }}</strong></span>
      <UIcon name="i-ph-plus-bold" />
      <span><UIcon name="i-ph-download-simple-bold" /><small>{{ t('payg.demo.download') }}</small><strong>{{ bytes(download) }}</strong></span>
    </div>
    <div class="traffic-graph__flow">
      <UIcon name="i-ph-devices-bold" /><i><b v-if="moving" :style="{ transform: `translateX(${packet * 1100}%)` }" /></i><span>{{ bytes(usageMiB) }}</span><i><b v-if="moving" :style="{ transform: `translateX(${packet * 1100}%)` }" /></i><UIcon name="i-ph-hard-drives-bold" />
    </div>
    <div class="traffic-graph__blocks">
      <div v-for="block in 2" :key="block" class="traffic-graph__block" :class="{ 'is-bought': blocks >= block, 'is-expired': expired }">
        <div><UIcon :name="blocks >= block ? 'i-ph-check-circle-fill' : 'i-ph-cube-bold'" /><span>{{ t('payg.demo.blockCapacity') }}</span></div>
        <div class="traffic-graph__bar"><i :style="{ transform: `scaleX(${used(block)})` }" /></div>
      </div>
    </div>
    <div class="traffic-graph__legend"><span><i />{{ t('payg.demo.used') }}</span><span><i />{{ t('payg.demo.credit') }}</span></div>
  </div>
</template>

<style scoped>
.traffic-graph { display: grid; gap: .9rem; min-width: 0; padding: 1rem .85rem; border: 1px solid var(--line); border-radius: var(--radius-panel); background: var(--surface); }
.traffic-graph__directions { display: flex; align-items: center; justify-content: center; gap: 1rem; color: var(--text-faint); }
.traffic-graph__directions > span { display: grid; grid-template-columns: auto 1fr; align-items: center; column-gap: .4rem; }
.traffic-graph__directions > span > .iconify { grid-row: span 2; font-size: 1.35rem; color: var(--accent); }
.traffic-graph__directions small { font-size: .62rem; }
.traffic-graph__directions strong { color: var(--text); font: 500 .83rem var(--font-mono); font-variant-numeric: tabular-nums; }
.traffic-graph__flow { display: flex; align-items: center; gap: .5rem; color: var(--accent); }
.traffic-graph__flow > .iconify { flex: none; font-size: 1.25rem; }
.traffic-graph__flow > span { flex: none; font: 600 1.3rem var(--font-mono); font-variant-numeric: tabular-nums; letter-spacing: -.04em; }
.traffic-graph__flow > i { flex: 1; height: 2px; background: var(--line); overflow: hidden; }
.traffic-graph__flow b { display: block; width: 9%; height: 2px; background: var(--accent); }
.traffic-graph__blocks { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: .65rem; }
.traffic-graph__block { min-width: 0; padding: .7rem; border: 1px dashed var(--line); border-radius: var(--radius-control); color: var(--text-faint); }
.traffic-graph__block > div:first-child { display: flex; align-items: center; gap: .4rem; font: .7rem var(--font-mono); }
.traffic-graph__block.is-bought { border-style: solid; border-color: color-mix(in srgb, var(--accent) 35%, var(--line)); color: var(--accent); }
.traffic-graph__bar { height: .65rem; margin-top: .6rem; overflow: hidden; border-radius: 1rem; background: var(--line); }
.is-bought .traffic-graph__bar { background: var(--accent); }
.traffic-graph__bar > i { display: block; width: 100%; height: 100%; background: var(--text-muted); transform-origin: left; }
.is-expired .traffic-graph__bar { background: var(--line); }
.traffic-graph__legend { display: flex; justify-content: center; gap: 1rem; color: var(--text-faint); font-size: .6rem; }
.traffic-graph__legend > span { display: inline-flex; align-items: center; gap: .3rem; }
.traffic-graph__legend i { width: .45rem; height: .45rem; border-radius: 50%; background: var(--text-muted); }
.traffic-graph__legend > span:last-child i { background: var(--accent); }
@media (max-width: 380px) { .traffic-graph { padding: .85rem .65rem; } .traffic-graph__directions { gap: .65rem; } .traffic-graph__flow > span { font-size: 1.12rem; } }
</style>
