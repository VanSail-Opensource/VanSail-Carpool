<script setup lang="ts">
import { nextTick, onScopeDispose, shallowRef, watch } from 'vue'
import { useMotionPreferences } from '@/composables/useMotionPreferences'
import { useI18n } from '@/i18n'

const props = withDefaults(defineProps<{ mode: 'payg' | 'combo'; ready?: boolean }>(), { ready: true })
const emit = defineEmits<{ active: [value: boolean] }>()
const { t } = useI18n()
const { reducedMotion } = useMotionPreferences()
const visible = shallowRef(false)
const bounds = shallowRef({ top: '0px', bottom: '0px', left: '0px' })
let timer: ReturnType<typeof globalThis.setTimeout> | undefined
let generation = 0

function reserveControls(): boolean {
  const height = globalThis.window.innerHeight
  const rectangles = (selector: string) => [...globalThis.document.querySelectorAll(selector)]
    .map(element => element.getBoundingClientRect()).filter(rectangle => rectangle.width > 0 && rectangle.height > 0)
  const top = Math.max(0, ...rectangles('.catalog-experience__header, .payg-summary__wallet').map(rectangle => rectangle.bottom))
  const lowerControls = rectangles('.bottom-nav, .catalog-flow-controls').filter(rectangle => rectangle.top >= top && rectangle.top < height)
  const bottom = Math.min(height, ...lowerControls.map(rectangle => rectangle.top))
  const left = Math.max(0, ...rectangles('.side-rail').map(rectangle => rectangle.right))
  bounds.value = { top: `${top}px`, bottom: `${height - bottom}px`, left: `${left}px` }
  return bottom - top >= 160
}

function stop(): void {
  if (timer) globalThis.clearTimeout(timer)
  globalThis.window.removeEventListener('resize', stop)
  globalThis.window.removeEventListener('scroll', stop, true)
  visible.value = false
  emit('active', false)
}

watch([() => props.mode, () => props.ready, reducedMotion], async () => {
  const ticket = ++generation
  stop()
  if (reducedMotion.value || !props.ready) return
  await nextTick()
  if (ticket !== generation) return
  if (!reserveControls()) return
  visible.value = true
  emit('active', true)
  timer = globalThis.setTimeout(stop, 800)
  globalThis.window.addEventListener('resize', stop)
  globalThis.window.addEventListener('scroll', stop, true)
}, { immediate: true })
onScopeDispose(() => { generation++; stop() })
</script>

<template>
  <Teleport to="body">
    <div v-if="visible" :key="mode" class="mode-transition" :style="bounds" aria-hidden="true">
      <div class="mode-transition__scene">
        <div class="mode-transition__icons" aria-hidden="true">
          <UIcon class="mode-transition__leading" :name="mode === 'payg' ? 'i-ph-coins' : 'i-ph-globe-hemisphere-east'" />
          <UIcon class="mode-transition__primary" :name="mode === 'payg' ? 'i-ph-wallet' : 'i-ph-stack'" />
          <UIcon class="mode-transition__trailing" :name="mode === 'payg' ? 'i-ph-paper-plane-tilt' : 'i-ph-compass'" />
        </div>
        <strong>{{ t(mode === 'payg' ? 'payg.automatic' : 'payg.comboIntro') }}</strong>
      </div>
    </div>
  </Teleport>
</template>

<style scoped>
.mode-transition { position: fixed; right: 0; z-index: 1000; display: grid; place-items: center; padding: 1.5rem; background: var(--bg, var(--surface)); color: var(--text); pointer-events: none; animation: mode-cover 800ms ease both; }
.mode-transition__scene { display: grid; justify-items: center; gap: 1rem; text-align: center; animation: mode-scene 800ms ease both; }
.mode-transition__scene strong { font-size: clamp(1.05rem, 4.6vw, 2rem); font-weight: 650; line-height: 1.4; letter-spacing: -.02em; white-space: nowrap; animation: mode-headline 650ms ease both; }
.mode-transition__icons { display: flex; align-items: center; gap: 1.25rem; font-size: 1.5rem; color: var(--text-muted); }
.mode-transition__primary { font-size: 3rem; color: var(--accent); animation: mode-icon 650ms ease both; }
.mode-transition__leading { animation: mode-leading 600ms ease both; }
.mode-transition__trailing { animation: mode-trailing 700ms ease both; }
@keyframes mode-cover { 0%, 40% { opacity: 1; } 100% { opacity: 0; } }
@keyframes mode-scene { 0% { opacity: 0; transform: translateY(8px); } 20%, 55% { opacity: 1; transform: translateY(0); } 100% { opacity: 0; transform: translateY(-8px); } }
@keyframes mode-headline { from { opacity: 0; transform: translateY(10px) scale(.98); } 55%, 100% { opacity: 1; transform: translateY(0) scale(1); } }
@keyframes mode-icon { 0% { opacity: .2; transform: scale(.75) rotate(-8deg); } 65%, 100% { opacity: 1; transform: scale(1) rotate(0); } }
@keyframes mode-leading { from { opacity: 0; transform: translateX(-16px) translateY(6px); } 60%, 100% { opacity: .8; transform: translateX(0) translateY(0); } }
@keyframes mode-trailing { 0%, 15% { opacity: 0; transform: translateX(-12px) translateY(6px); } 80%, 100% { opacity: .8; transform: translateX(0) translateY(0); } }
@media (prefers-reduced-motion: reduce) { .mode-transition { display: none; animation: none; } .mode-transition * { animation: none; } }
</style>
