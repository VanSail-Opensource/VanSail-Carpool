<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from '@/i18n'
import DemoPointer from '@/components/common/DemoPointer.vue'
import { between, interactionCue } from '@/components/common/illustrationTimeline'

const props = defineProps<{ step: number; phase: number; matches: boolean; reducedMotion: boolean; description: string }>()
const { t } = useI18n()
const fields = [
  { key: 'entry', icon: 'i-ph-sign-in', at: .22 },
  { key: 'template', icon: 'i-ph-funnel', at: .47 },
  { key: 'exit', icon: 'i-ph-sign-out', at: .72 },
] as const
const checks = [
  { key: 'access', icon: 'i-ph-key', at: .2 },
  { key: 'noLoops', icon: 'i-ph-arrows-clockwise', at: .42 },
  { key: 'valid', icon: 'i-ph-shield-check', at: .64 },
] as const
const applied = computed(() => props.step > 2 || (props.step === 2 && props.phase >= .76))
const queued = computed(() => props.step === 2 && props.phase >= .26 && !applied.value)
const cue = computed(() => interactionCue(props.phase, props.step === 0 ? [{ at: .22, x: 17, y: 6.4 }, { at: .47, x: 50, y: 6.4 }, { at: .72, x: 83, y: 6.4 }] : props.step === 2 ? [{ at: .26, x: 87, y: 92 }] : []))
const nodes = computed(() => [
  { key: 'you', icon: 'i-ph-device-mobile', x: 10, y: 52, active: applied.value },
  { key: 'entry', icon: 'i-ph-computer-tower', x: 33, y: 52, active: applied.value },
  { key: 'exit', icon: 'i-ph-hard-drives', x: 62, y: 23, active: applied.value && props.matches },
  { key: 'default', icon: 'i-ph-arrow-elbow-right', x: 62, y: 81, active: applied.value && !props.matches },
  { key: 'destination', icon: 'i-ph-globe', x: 90, y: 52, active: applied.value },
])
const packet = computed(() => {
  const p = props.reducedMotion ? .5 : props.phase * 3 % 1
  const points = [[10, 52], [33, 52], [62, props.matches ? 23 : 81], [90, 52]]
  const index = Math.min(2, Math.floor(p * 3))
  const a = points[index]!, b = points[index + 1]!, part = p * 3 - index
  return { x: a[0]! + (b[0]! - a[0]!) * part, y: a[1]! + (b[1]! - a[1]!) * part }
})
</script>

<template>
  <div class="routing-scene" role="img" :aria-label="description" :data-demo-applied="applied" :data-demo-match="matches">
    <div class="routing-scene__fields" aria-hidden="true">
      <span v-for="field in fields" :key="field.key" :class="{ 'is-selected': step > 0 || phase >= field.at }"><UIcon :name="field.icon" /><span>{{ t(`diy.demo.nodes.${field.key}`) }}</span><UIcon name="i-ph-caret-down-bold" /></span>
    </div>
    <div class="routing-scene__graph" aria-hidden="true">
      <svg class="routing-scene__paths" viewBox="0 0 100 100" preserveAspectRatio="none">
        <path d="M 10 52 H 33" :class="{ 'is-active': applied }" />
        <path d="M 33 52 L 62 23 L 90 52" :class="{ 'is-active': applied && matches }" />
        <path d="M 33 52 L 62 81 L 90 52" :class="{ 'is-active': applied && !matches }" />
        <circle v-if="applied" :cx="packet.x" :cy="packet.y" r="1.1" class="routing-scene__packet" />
      </svg>
      <div v-for="node in nodes" :key="node.key" class="routing-scene__node" :class="{ 'is-active': node.active }" :style="{ left: `${node.x}%`, top: `${node.y}%` }"><span><UIcon :name="node.icon" /></span><small>{{ t(`diy.demo.nodes.${node.key}`) }}</small></div>
      <span class="routing-scene__rule"><UIcon name="i-ph-funnel" />{{ t('diy.demo.firstMatch') }}</span>
    </div>
    <div class="routing-scene__footer" aria-hidden="true">
      <div class="routing-scene__checks"><span v-for="check in checks" :key="check.key" :class="{ 'is-checked': step > 1 || (step === 1 && phase >= check.at) }"><UIcon :name="check.icon" /><UIcon name="i-ph-check-circle-fill" class="routing-scene__check" /></span></div>
      <span class="routing-scene__save" :class="{ 'is-applied': applied }"><UIcon :name="applied ? 'i-ph-check-circle-fill' : queued ? 'i-ph-clock-countdown' : 'i-ph-floppy-disk'" /><span>{{ t(`diy.demo.nodes.${applied ? 'applied' : queued ? 'queue' : 'save'}`) }}</span><i v-if="queued" :style="{ transform: `scaleX(${between(phase, .26, .76)})` }" /></span>
    </div>
    <DemoPointer :cue="cue" :reduced-motion="reducedMotion" />
  </div>
</template>

<style scoped>
.routing-scene { position: relative; display: grid; grid-template-rows: 52px 214px 48px; min-width: 0; overflow: hidden; }
.routing-scene__fields { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); align-items: start; gap: .5rem; }
.routing-scene__fields > span { display: flex; align-items: center; gap: .35rem; min-width: 0; padding: .65rem .55rem; border: 1px solid var(--line); border-radius: var(--radius-control); color: var(--text-faint); font-size: .72rem; }
.routing-scene__fields > span > .iconify { flex: none; }
.routing-scene__fields > span > span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.routing-scene__fields > span > .iconify:last-child { margin-inline-start: auto; font-size: .6rem; }
.routing-scene__fields > span.is-selected { border-color: color-mix(in srgb, var(--accent) 50%, var(--line)); color: var(--accent); background: var(--accent-soft); }
.routing-scene__graph { position: relative; min-width: 0; }
.routing-scene__paths { position: absolute; inset: 0; width: 100%; height: 100%; }
.routing-scene__paths path { fill: none; stroke: var(--line); stroke-width: 1.4; vector-effect: non-scaling-stroke; }
.routing-scene__paths path.is-active { stroke: var(--accent); }
.routing-scene__packet { fill: var(--text); }
.routing-scene__node { position: absolute; display: grid; justify-items: center; gap: .3rem; width: 4.2rem; transform: translate(-50%, -50%); color: var(--text-muted); text-align: center; }
.routing-scene__node > span { display: grid; place-items: center; width: 2.6rem; height: 2.6rem; border: 1px solid var(--line); border-radius: .65rem; background: var(--surface); color: var(--text-muted); font-size: 1.25rem; }
.routing-scene__node > small { font-size: .72rem; }
.routing-scene__node.is-active > span { color: var(--accent); border-color: var(--accent); background: var(--surface-raised); }
.routing-scene__rule { position: absolute; top: .1rem; left: 0; display: flex; align-items: center; gap: .3rem; color: var(--text-faint); font-size: .65rem; }
.routing-scene__footer { display: flex; align-items: center; justify-content: space-between; gap: 1rem; border-top: 1px solid var(--line); }
.routing-scene__checks { display: flex; gap: .7rem; }
.routing-scene__checks > span { position: relative; color: var(--text-faint); font-size: 1.15rem; }
.routing-scene__check { position: absolute; bottom: -.15rem; right: -.25rem; font-size: .6rem; color: var(--success); opacity: 0; }
.routing-scene__checks .is-checked { color: var(--text); }
.routing-scene__checks .is-checked .routing-scene__check { opacity: 1; }
.routing-scene__save { position: relative; display: flex; align-items: center; justify-content: center; gap: .4rem; min-width: 5.5rem; padding: .5rem .75rem; overflow: hidden; border: 1px solid var(--line); border-radius: .45rem; color: var(--text-muted); font-size: .72rem; }
.routing-scene__save.is-applied { color: var(--success); }
.routing-scene__save > i { position: absolute; bottom: 0; left: 0; right: 0; height: 2px; background: var(--accent); transform-origin: left; }
@media (max-width: 380px) { .routing-scene__fields { gap: .25rem; } .routing-scene__fields > span { gap: .2rem; padding-inline: .35rem; } .routing-scene__fields > span > .iconify:last-child { display: none; } .routing-scene__node > span { width: 2.25rem; height: 2.25rem; } .routing-scene__node { width: 3.5rem; } }
</style>
