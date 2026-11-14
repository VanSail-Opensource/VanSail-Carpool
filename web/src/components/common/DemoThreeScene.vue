<script setup lang="ts">
import { computed, onMounted, onScopeDispose, shallowRef, useTemplateRef, watch } from 'vue'
import { demoThreeActive, demoThreeCapacityScale, demoThreeRouteState, demoThreeStory, demoThreeSymbols, type DemoThreeFrame } from './demoThreeModel'
import type { DemoThreeRenderer } from './demoThreeRenderer'

const props = defineProps<DemoThreeFrame>()
const canvas = useTemplateRef<globalThis.HTMLCanvasElement>('canvas')
const status = shallowRef<'loading' | 'ready' | 'fallback'>('loading')
const story = computed(() => demoThreeStory(props.topic))
const active = computed(() => demoThreeActive(props))
const signature = computed(() => `${props.topic}:${props.step}:${Math.round(props.phase * 1000)}:${props.reducedMotion}:${props.matches ?? true}`)
const paths = computed(() => story.value.routes.map(route => {
  const from = story.value.nodes.find(node => node.id === route.from)!, to = story.value.nodes.find(node => node.id === route.to)!
  const midX = (from.x + to.x) / 2, midY = -(from.y + to.y) / 2 - (route.bend ?? 0), at = .76
  const stopX = (1 - at) ** 2 * from.x + 2 * (1 - at) * at * midX + at ** 2 * to.x
  const stopY = (1 - at) ** 2 * -from.y + 2 * (1 - at) * at * midY + at ** 2 * -to.y
  return { key: `${route.from}-${route.to}`, path: `M ${from.x} ${-from.y} Q ${midX} ${midY} ${to.x} ${-to.y}`, stop: `M ${stopX - .1} ${stopY - .1} l .2 .2 m 0 -.2 l -.2 .2`, active: demoThreeRouteState(props, route).highlighted, warning: route.warning }
}))
let renderer: DemoThreeRenderer | undefined
let observer: globalThis.ResizeObserver | undefined
let generation = 0, disposed = false

function release(): void {
  const previous = renderer
  renderer = undefined
  previous?.dispose()
}
function draw(): void {
  if (disposed || status.value !== 'ready') return
  try { renderer?.draw(props) } catch { status.value = 'fallback'; release() }
}
async function start(): Promise<void> {
  const target = canvas.value, request = ++generation
  if (!target || disposed) return
  status.value = 'loading'
  try {
    const { createDemoThreeRenderer } = await import('./demoThreeRenderer')
    if (disposed || request !== generation) return
    const next = await createDemoThreeRenderer(target, props.topic)
    if (disposed || request !== generation) { next.dispose(); return }
    renderer = next; status.value = 'ready'; draw()
  } catch { if (!disposed && request === generation) status.value = 'fallback' }
}
function lost(event: globalThis.Event): void {
  event.preventDefault()
  ++generation
  if (!disposed) status.value = 'fallback'
}
function restored(): void {
  // Three rebuilds its GL state in its own listener before this microtask runs.
  const request = generation
  globalThis.queueMicrotask(() => { if (!disposed && renderer && request === generation) { status.value = 'ready'; draw() } })
}

watch(() => [props.topic, props.step, props.phase, props.reducedMotion, props.matches], draw, { flush: 'post' })
onMounted(() => {
  if (typeof globalThis.ResizeObserver !== 'undefined' && canvas.value) { observer = new globalThis.ResizeObserver(draw); observer.observe(canvas.value) }
  void start()
})
onScopeDispose(() => { disposed = true; ++generation; observer?.disconnect(); release() })
</script>

<template>
  <div class="demo-three-scene" :data-three-status="status" :data-three-topic="topic" :data-three-frame="signature" aria-hidden="true">
    <canvas ref="canvas" class="demo-three-scene__canvas" :class="{ 'is-ready': status === 'ready' }" @webglcontextlost="lost" @webglcontextrestored="restored" />
    <svg v-if="status !== 'ready'" class="demo-three-scene__fallback" viewBox="-5 -2.8 10 5.6" fill="none">
      <g v-for="path in paths" :key="path.key">
        <path :d="path.path" class="demo-three-scene__path" :class="{ 'is-active': path.active, 'is-warning': path.warning }" />
        <path v-if="path.warning && path.active" :d="path.stop" class="demo-three-scene__stop" />
      </g>
      <g v-for="node in story.nodes" :key="node.id" :transform="`translate(${node.x} ${-node.y}) scale(${(node.scale ?? 1) * .05 * demoThreeCapacityScale(props, node.id)}) translate(-12 -12)`" class="demo-three-scene__symbol" :class="{ 'is-active': active.includes(node.id), 'is-warning': node.warning }">
        <rect x="-4" y="-4" width="32" height="32" rx="6" class="demo-three-scene__tile" />
        <path :d="demoThreeSymbols[node.shape]" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" />
      </g>
    </svg>
    <slot />
  </div>
</template>

<style scoped>
.demo-three-scene { position: relative; width: 100%; min-width: 0; aspect-ratio: 1.8; overflow: hidden; isolation: isolate; }
.demo-three-scene__canvas, .demo-three-scene__fallback { position: absolute; inset: 0; width: 100%; height: 100%; display: block; pointer-events: none; }
.demo-three-scene__canvas { visibility: hidden; }
.demo-three-scene__canvas.is-ready { visibility: visible; }
.demo-three-scene__path { stroke: var(--line); stroke-width: .025; }
.demo-three-scene__path.is-active { stroke: var(--accent); stroke-width: .035; }
.demo-three-scene__path.is-warning { stroke-dasharray: .08 .08; }
.demo-three-scene__stop { stroke: var(--warning); stroke-width: .055; stroke-linecap: round; }
.demo-three-scene__symbol { color: var(--text-faint); }
.demo-three-scene__symbol.is-active { color: var(--accent); }
.demo-three-scene__tile { fill: var(--surface-raised, var(--surface)); stroke: var(--line); stroke-width: .75; }
.demo-three-scene__symbol.is-active .demo-three-scene__tile { stroke: var(--accent-line); }
.demo-three-scene__symbol.is-warning.is-active, .demo-three-scene__path.is-warning.is-active { color: var(--warning); stroke: var(--warning); }
</style>
