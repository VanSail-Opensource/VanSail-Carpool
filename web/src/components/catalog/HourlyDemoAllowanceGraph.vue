<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from '@/i18n'

const props = defineProps<{ remaining: number; enabled: boolean; extra: boolean; reduced: boolean; phase: number; moving: boolean; reducedMotion: boolean }>()
const { t } = useI18n()
const pool = computed(() => props.extra ? 'extraBlock' : 'dailyPool')
const packet = computed(() => props.reducedMotion ? .5 : props.phase * (props.reduced ? .5 : 2) % 1)
</script>

<template>
  <div class="hourly-graph" :class="{ 'is-reduced': reduced, 'has-extra': extra, 'is-off': !enabled }" data-demo-graph="hourly">
    <div class="hourly-graph__sources"><span v-for="index in 3" :key="index"><UIcon name="i-ph-hard-drives-bold" /><i><b v-if="moving" :style="{ transform: `translateY(${packet * 700}%)` }" /></i></span></div>
    <div class="hourly-graph__pool">
      <div class="hourly-graph__heading"><span><UIcon :name="extra ? 'i-ph-rocket-launch-bold' : 'i-ph-lightning-bold'" />{{ t(`payg.demo.${pool}`) }}</span><strong>{{ Math.round(remaining) }}<small>%</small></strong></div>
      <div class="hourly-graph__meter"><i :style="{ transform: `scaleX(${remaining / 100})` }" /></div>
      <span class="hourly-graph__shared"><UIcon name="i-ph-intersect-bold" />{{ t('payg.demo.shared') }}</span>
    </div>
    <div class="hourly-graph__speed"><UIcon name="i-ph-gauge-bold" /><div><i v-for="index in 12" :key="index" :class="{ 'is-lit': index <= (enabled ? reduced ? 3 : 12 : 0) }" /></div><span>{{ t(`payg.demo.${!enabled ? 'notEnabled' : reduced ? 'reduced' : 'fullSpeed'}`) }}</span></div>
  </div>
</template>

<style scoped>
.hourly-graph { display: grid; gap: .75rem; padding: .65rem .85rem .9rem; min-width: 0; border: 1px solid var(--line); border-radius: var(--radius-panel); background: var(--surface); --pool: var(--accent); }
.hourly-graph__sources { display: flex; justify-content: space-evenly; color: var(--text-muted); }
.hourly-graph__sources > span { display: grid; justify-items: center; gap: .35rem; font-size: 1.2rem; }
.hourly-graph__sources i { width: 2px; height: 1.3rem; overflow: hidden; background: var(--line); }
.hourly-graph__sources b { display: block; width: 2px; height: 14%; background: var(--pool); }
.hourly-graph__pool { display: grid; gap: .65rem; padding: .85rem; border: 1px solid color-mix(in srgb, var(--pool) 30%, var(--line)); border-radius: var(--radius-control); background: var(--surface-soft); }
.hourly-graph__heading { display: flex; align-items: center; justify-content: space-between; gap: .5rem; color: var(--pool); }
.hourly-graph__heading > span { display: flex; align-items: center; gap: .4rem; font-size: .75rem; }
.hourly-graph__heading > strong { font: 500 1.6rem var(--font-mono); font-variant-numeric: tabular-nums; letter-spacing: -.05em; }
.hourly-graph__heading small { margin-left: .1rem; font-size: .8rem; }
.hourly-graph__meter { height: .65rem; overflow: hidden; border-radius: 1rem; background: var(--line); }
.hourly-graph__meter > i { display: block; width: 100%; height: 100%; transform-origin: left; background: var(--pool); }
.hourly-graph__shared { display: flex; justify-content: center; align-items: center; gap: .3rem; color: var(--text-faint); font-size: .62rem; }
.hourly-graph__speed { display: flex; align-items: center; gap: .5rem; color: var(--pool); font-size: .64rem; }
.hourly-graph__speed > .iconify { flex: none; font-size: 1.15rem; }
.hourly-graph__speed > div { display: flex; flex: 1; gap: .2rem; min-width: 0; }
.hourly-graph__speed i { flex: 1; height: .5rem; border-radius: .15rem; background: var(--line); }
.hourly-graph__speed i.is-lit { background: var(--pool); }
.hourly-graph__speed > span { max-width: 38%; text-align: right; }
.has-extra { --pool: var(--success); }
.is-reduced { --pool: var(--warning); }
.is-off .hourly-graph__speed { color: var(--text-faint); }
</style>
