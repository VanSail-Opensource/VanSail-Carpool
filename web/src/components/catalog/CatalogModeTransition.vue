<script setup lang="ts">
import { nextTick, onScopeDispose, shallowRef, watch } from 'vue'
import { useMotionPreferences } from '@/composables/useMotionPreferences'
import { useI18n } from '@/i18n'
import { modeTransitionMask } from './modeTransitionMask'

const props = defineProps<{ mode: 'payg' | 'combo' }>()
const emit = defineEmits<{ active: [value: boolean] }>()
const { t } = useI18n()
const { reducedMotion } = useMotionPreferences()
const visible = shallowRef(false)
const clipPath = shallowRef('')
let timer: ReturnType<typeof globalThis.setTimeout> | undefined
let observer: globalThis.MutationObserver | undefined
let generation = 0

function protectControls(): void {
  const controls = [...globalThis.document.querySelectorAll('button, a, summary, [role="button"], [role="tab"], [role="combobox"]')]
    .map(element => element.getBoundingClientRect()).filter(rectangle => rectangle.width > 0 && rectangle.height > 0)
  clipPath.value = modeTransitionMask(globalThis.window.innerWidth, globalThis.window.innerHeight, controls)
}

function stop(): void {
  if (timer) globalThis.clearTimeout(timer)
  observer?.disconnect()
  globalThis.window.removeEventListener('resize', protectControls)
  globalThis.window.removeEventListener('scroll', protectControls, true)
  visible.value = false
  emit('active', false)
}

watch([() => props.mode, reducedMotion], async () => {
  const ticket = ++generation
  stop()
  if (reducedMotion.value) return
  if (!globalThis.CSS?.supports?.('clip-path', 'path(evenodd, "M0 0H1V1H0Z")')) return
  await nextTick()
  if (ticket !== generation) return
  protectControls()
  visible.value = true
  emit('active', true)
  timer = globalThis.setTimeout(stop, 800)
  observer = new globalThis.MutationObserver(protectControls)
  observer.observe(globalThis.document.body, { childList: true, subtree: true })
  globalThis.window.addEventListener('resize', protectControls)
  globalThis.window.addEventListener('scroll', protectControls, true)
}, { immediate: true })
onScopeDispose(() => { generation++; stop() })
</script>

<template>
  <Teleport to="body">
    <div v-if="visible" :key="mode" class="mode-transition" :style="{ clipPath }" aria-hidden="true">
      <div class="mode-transition__scene">
        <strong>{{ t(mode === 'payg' ? 'payg.headline' : 'payg.comboHeadline') }}</strong>
        <p>{{ t(mode === 'payg' ? 'payg.automatic' : 'payg.comboIntro') }}</p>
      </div>
    </div>
  </Teleport>
</template>

<style scoped>
.mode-transition { position: fixed; inset: 0; z-index: 1000; display: grid; place-items: center; padding: 1.5rem; background: color-mix(in srgb, var(--surface) 94%, transparent); color: var(--text); pointer-events: none; animation: mode-cover 800ms ease both; }
.mode-transition__scene { display: grid; justify-items: center; gap: .65rem; text-align: center; animation: mode-scene 800ms ease both; }
.mode-transition__scene strong { max-width: 14ch; font-size: clamp(1.7rem, 6vw, 2.8rem); font-weight: 650; line-height: 1.2; letter-spacing: -.035em; text-wrap: balance; animation: mode-headline 650ms ease both; }
.mode-transition__scene p { margin: 0; color: var(--text-muted); font-size: .85rem; line-height: 1.5; }
@keyframes mode-cover { 0% { opacity: 0; } 15%, 40% { opacity: 1; } 100% { opacity: 0; } }
@keyframes mode-scene { 0% { opacity: 0; transform: translateY(8px); } 20%, 55% { opacity: 1; transform: translateY(0); } 100% { opacity: 0; transform: translateY(-8px); } }
@keyframes mode-headline { from { opacity: 0; transform: translateY(10px) scale(.98); } 55%, 100% { opacity: 1; transform: translateY(0) scale(1); } }
@media (prefers-reduced-motion: reduce) { .mode-transition { display: none; animation: none; } .mode-transition * { animation: none; } }
</style>
