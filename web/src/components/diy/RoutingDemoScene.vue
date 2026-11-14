<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from '@/i18n'

const props = defineProps<{ step: number; matches: boolean; playing: boolean; description: string }>()
const { t } = useI18n()
const scenes = [
  [{ icon: 'i-ph-computer-tower', label: 'entry' }, { icon: 'i-ph-funnel', label: 'template' }, { icon: 'i-ph-hard-drives', label: 'exit' }],
  [{ icon: 'i-ph-key', label: 'access' }, { icon: 'i-ph-arrows-clockwise', label: 'noLoops' }, { icon: 'i-ph-shield-check', label: 'valid' }],
  [{ icon: 'i-ph-floppy-disk', label: 'save' }, { icon: 'i-ph-clock-countdown', label: 'queue' }, { icon: 'i-ph-check-circle', label: 'applied' }],
] as const
const scene = computed(() => scenes[props.step])
const nodes = computed(() => [
  { key: 'you', icon: 'i-ph-device-mobile', left: '10%', top: '50%', active: true },
  { key: 'entry', icon: 'i-ph-computer-tower', left: '34%', top: '50%', active: true },
  { key: 'exit', icon: 'i-ph-hard-drives', left: '63%', top: '20%', active: props.matches },
  { key: 'default', icon: 'i-ph-arrow-elbow-right', left: '63%', top: '80%', active: !props.matches },
  { key: 'destination', icon: 'i-ph-globe', left: '90%', top: '50%', active: true },
])
</script>

<template>
  <div class="routing-scene" :class="{ 'is-playing': playing }" role="img" :aria-label="description">
    <Transition name="routing-scene" mode="out-in">
      <div v-if="scene" :key="step" class="routing-scene-pipeline" aria-hidden="true">
        <template v-for="(item, index) in scene" :key="item.label">
          <span v-if="index" class="routing-scene-link"><UIcon name="i-ph-arrow-right" /></span>
          <div class="routing-scene-item" :style="{ animationDelay: `${index * .45}s` }">
            <span class="routing-scene-symbol"><UIcon :name="item.icon" /><UIcon v-if="step === 1" name="i-ph-check-circle-fill" class="routing-scene-check" /></span>
            <span>{{ t(`diy.demo.nodes.${item.label}`) }}</span>
          </div>
        </template>
      </div>
      <div v-else key="flow" class="routing-scene-graph" aria-hidden="true">
        <svg class="routing-scene-paths" viewBox="0 0 100 100" preserveAspectRatio="none">
          <path d="M 14 50 H 30" class="is-active" />
          <path d="M 38 49 C 49 49, 49 20, 59 20 M 67 20 C 80 20, 80 49, 86 49" :class="{ 'is-active': matches }" />
          <path d="M 38 51 C 49 51, 49 80, 59 80 M 67 80 C 80 80, 80 51, 86 51" :class="{ 'is-active': !matches }" />
        </svg>
        <span class="routing-scene-rule"><UIcon name="i-ph-funnel" />{{ t('diy.demo.firstMatch') }}</span>
        <div v-for="node in nodes" :key="node.key" class="routing-scene-node" :class="{ 'is-active': node.active }" :style="{ left: node.left, top: node.top }">
          <span class="routing-scene-symbol"><UIcon :name="node.icon" /></span><span>{{ t(`diy.demo.nodes.${node.key}`) }}</span>
        </div>
      </div>
    </Transition>
  </div>
</template>

<style scoped>
.routing-scene { min-width: 0; border-block: 1px solid var(--line); }
.routing-scene-pipeline { display: grid; grid-template-columns: minmax(0, 1fr) 1.1rem minmax(0, 1fr) 1.1rem minmax(0, 1fr); align-items: center; gap: .5rem; min-height: 250px; padding-inline: .5rem; }
.routing-scene-item { display: grid; place-items: center; align-content: center; gap: 1rem; color: var(--text-muted); font-size: .8rem; text-align: center; }
.routing-scene-symbol { position: relative; display: grid; place-items: center; width: 3.5rem; height: 3.5rem; border: 1px solid var(--line); border-radius: 1rem; background: var(--surface); color: var(--ui-text); font-size: 1.65rem; }
.routing-scene-check { position: absolute; right: -.3rem; bottom: -.3rem; color: var(--ui-success); background: var(--canvas); border-radius: 50%; font-size: 1.15rem; }
.routing-scene-link { color: var(--ui-primary); }
.routing-scene-graph { position: relative; height: 250px; }
.routing-scene-paths { position: absolute; width: 100%; height: 100%; }
.routing-scene-paths path { fill: none; stroke: var(--line); stroke-width: 1.5; vector-effect: non-scaling-stroke; }
.routing-scene-paths path.is-active { stroke: var(--ui-primary); stroke-width: 2; stroke-dasharray: 5 5; }
.routing-scene-node { position: absolute; display: grid; justify-items: center; gap: .5rem; width: 4.25rem; transform: translate(-50%, -50%); color: var(--text-muted); font-size: .7rem; text-align: center; }
.routing-scene-node .routing-scene-symbol { width: 2.75rem; height: 2.75rem; font-size: 1.3rem; border-radius: .8rem; }
.routing-scene-node.is-active .routing-scene-symbol { color: var(--ui-primary); border-color: var(--ui-primary); }
.routing-scene-rule { position: absolute; top: .75rem; left: 0; display: flex; gap: .35rem; align-items: center; font-size: .7rem; color: var(--text-muted); }
@media (prefers-reduced-motion: no-preference) {
  .routing-scene-item { animation: routing-node-pulse 2.4s ease-in-out infinite; animation-play-state: paused; }
  .routing-scene.is-playing .routing-scene-item { animation-play-state: running; }
  .routing-scene-paths path.is-active { animation: routing-packet 1.2s linear infinite; animation-play-state: paused; }
  .routing-scene.is-playing .routing-scene-paths path.is-active { animation-play-state: running; }
  .routing-scene-enter-active, .routing-scene-leave-active { transition: opacity .16s, transform .16s; }
  .routing-scene-enter-from { opacity: 0; transform: translateY(6px); }
  .routing-scene-leave-to { opacity: 0; transform: translateY(-6px); }
}
@keyframes routing-packet { to { stroke-dashoffset: -20; } }
@keyframes routing-node-pulse { 50% { transform: translateY(-4px); } }
</style>
