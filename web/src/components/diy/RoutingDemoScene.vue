<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from '@/i18n'
import { between } from '@/components/common/illustrationTimeline'
import RoutingFlowGraph from './RoutingFlowGraph.vue'

const props = defineProps<{ step: number; phase: number; matches: boolean; reducedMotion: boolean; description: string }>()
const { t } = useI18n()
const fields = [
  { key: 'entry', icon: 'i-ph-sign-in-bold' },
  { key: 'template', icon: 'i-ph-funnel-bold' },
  { key: 'exit', icon: 'i-ph-sign-out-bold' },
] as const
const checks = [
  { key: 'access', icon: 'i-ph-key-bold', at: .2 },
  { key: 'noLoops', icon: 'i-ph-arrows-clockwise-bold', at: .42 },
  { key: 'valid', icon: 'i-ph-shield-check-bold', at: .64 },
] as const
const applied = computed(() => props.step > 5 || (props.step === 5 && props.phase >= .7))
const queued = computed(() => (props.step === 4 && props.phase >= .3) || (props.step === 5 && !applied.value))
const refreshed = computed(() => props.step === 7 && props.phase >= .65)
const progress = computed(() => props.step === 4 ? between(props.phase, .3, 1) * .35 : props.step === 5 ? .35 + between(props.phase, 0, .7) * .65 : applied.value ? 1 : 0)
</script>

<template>
  <div class="routing-scene" role="img" :aria-label="description" data-demo-visual="routing" :data-demo-frame="`${step}:${phase.toFixed(3)}`" :data-demo-applied="applied" :data-demo-match="matches">
    <div class="routing-scene__fields" aria-hidden="true"><span v-for="(field, index) in fields" :key="field.key" :class="{ 'is-selected': step > index || (step === index && phase >= .35), 'is-current': step === index }"><UIcon :name="field.icon" /><span>{{ t(`diy.demo.nodes.${field.key}`) }}</span><UIcon :name="step > index || (step === index && phase >= .35) ? 'i-ph-check-bold' : 'i-ph-caret-down-bold'" /></span></div>
    <div class="routing-scene__graph" aria-hidden="true"><RoutingFlowGraph :step="step" :phase="phase" :matches="matches" :applied="applied" :running="step >= 6" :reduced-motion="reducedMotion" /></div>
    <div class="routing-scene__checks" aria-hidden="true"><span v-for="check in checks" :key="check.key" :class="{ 'is-checked': step > 3 || (step === 3 && phase >= check.at) }"><UIcon :name="check.icon" /><span>{{ t(`diy.demo.nodes.${check.key}`) }}</span><UIcon name="i-ph-check-circle-fill" class="routing-scene__check" /></span></div>
    <div class="routing-scene__footer" aria-hidden="true"><span><UIcon name="i-ph-list-numbers-bold" />{{ t('diy.demo.firstMatch') }}</span><span class="routing-scene__save" :class="{ 'is-applied': applied }"><UIcon :name="applied ? 'i-ph-check-circle-fill' : queued ? 'i-ph-clock-countdown-bold' : 'i-ph-floppy-disk-bold'" />{{ t(`diy.demo.nodes.${applied ? 'applied' : queued ? 'queue' : 'save'}`) }}<i :style="{ transform: `scaleX(${progress})` }" /></span></div>
    <div class="routing-scene__refresh" :class="{ 'is-current': step === 7 }" aria-hidden="true"><UIcon :name="refreshed ? 'i-ph-check-circle-fill' : 'i-ph-arrows-clockwise-bold'" :style="{ transform: step === 7 && !refreshed && !reducedMotion ? `rotate(${phase * 360}deg)` : undefined }" /><span>{{ t(`diy.demo.nodes.${refreshed ? 'refreshed' : 'refresh'}`) }}</span></div>
  </div>
</template>

<style scoped>
.routing-scene { display: grid; gap: .6rem; min-width: 0; padding: .85rem; overflow: hidden; }
.routing-scene__fields { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: .45rem; }
.routing-scene__fields > span { display: flex; align-items: center; gap: .3rem; min-width: 0; padding: .65rem .5rem; border: 1px solid var(--line); border-radius: var(--radius-control); color: var(--text-faint); font-size: .7rem; }
.routing-scene__fields > span > .iconify { flex: none; }
.routing-scene__fields > span > span { overflow-wrap: anywhere; }
.routing-scene__fields > span > .iconify:last-child { margin-inline-start: auto; font-size: .6rem; }
.routing-scene__fields > span.is-selected { border-color: color-mix(in srgb, var(--accent) 40%, var(--line)); color: var(--accent); }
.routing-scene__fields > span.is-current { background: var(--accent-soft); }
.routing-scene__graph { min-width: 0; border: 1px solid var(--line); border-radius: var(--radius-panel); background: var(--surface); }
.routing-scene__checks { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: .4rem; }
.routing-scene__checks > span { display: flex; align-items: center; gap: .25rem; min-width: 0; color: var(--text-faint); font-size: .62rem; }
.routing-scene__checks .iconify { flex: none; }
.routing-scene__check { margin-inline-start: auto; opacity: 0; color: var(--success); }
.routing-scene__checks .is-checked { color: var(--text-muted); }
.routing-scene__checks .is-checked .routing-scene__check { opacity: 1; }
.routing-scene__footer { display: flex; align-items: center; justify-content: space-between; gap: .5rem; padding-top: .6rem; border-top: 1px solid var(--line); }
.routing-scene__footer > span { display: flex; align-items: center; gap: .35rem; color: var(--text-muted); font-size: .65rem; }
.routing-scene__footer > span > .iconify { flex: none; }
.routing-scene__save { position: relative; justify-content: center; min-width: 5.5rem; padding: .55rem .65rem; overflow: hidden; border: 1px solid var(--line); border-radius: var(--radius-control); }
.routing-scene__save.is-applied { color: var(--success); }
.routing-scene__save > i { position: absolute; bottom: 0; left: 0; right: 0; height: 2px; background: var(--accent); transform-origin: left; }
.routing-scene__refresh { display: flex; align-items: center; justify-content: center; gap: .4rem; min-height: 1.6rem; color: var(--text-faint); font-size: .65rem; }
.routing-scene__refresh.is-current { color: var(--accent); }
@media (max-width: 420px) { .routing-scene { padding: .65rem; } .routing-scene__fields { gap: .25rem; } .routing-scene__fields > span { gap: .2rem; padding-inline: .35rem; } .routing-scene__fields > span > .iconify:last-child { display: none; } }
</style>
