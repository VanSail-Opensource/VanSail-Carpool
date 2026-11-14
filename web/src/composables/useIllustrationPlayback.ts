import { computed, onMounted, onScopeDispose, shallowRef, toValue, watch, type MaybeRefOrGetter } from 'vue'
import { useMotionPreferences } from './useMotionPreferences'

/** A local video clock. No account data, network work or server mutations. */
export function useIllustrationPlayback(options: {
  active: MaybeRefOrGetter<boolean>
  stepCount: MaybeRefOrGetter<number>
  resetKey?: MaybeRefOrGetter<unknown>
  interval?: MaybeRefOrGetter<number>
}) {
  const { reducedMotion } = useMotionPreferences()
  const elapsed = shallowRef(0)
  const requested = shallowRef(false)
  const hidden = shallowRef(typeof document !== 'undefined' && document.hidden)
  const count = computed(() => Math.max(1, Math.trunc(toValue(options.stepCount)) || 1))
  const interval = computed(() => Math.max(1, toValue(options.interval) ?? 4200))
  const duration = computed(() => count.value * interval.value)
  const step = computed(() => Math.min(count.value - 1, Math.floor(elapsed.value / interval.value)))
  const phase = computed(() => Math.min(1, (elapsed.value - step.value * interval.value) / interval.value))
  const progress = computed(() => elapsed.value / duration.value)
  const playing = computed(() => toValue(options.active) && requested.value && !hidden.value && !reducedMotion.value && elapsed.value < duration.value)
  let frame: number | undefined
  let previousTime: number | undefined

  function clear() {
    if (frame !== undefined) cancelAnimationFrame(frame)
    frame = undefined
    previousTime = undefined
  }
  function tick(now: number) {
    frame = undefined
    if (!playing.value) return
    elapsed.value = Math.min(duration.value, elapsed.value + Math.max(0, now - (previousTime ?? now)))
    previousTime = now
    if (playing.value) frame = requestAnimationFrame(tick)
    else requested.value = false
  }
  function schedule() {
    clear()
    if (!playing.value) return
    previousTime = performance.now()
    frame = requestAnimationFrame(tick)
  }
  function pause() { requested.value = false }
  function play() {
    if (!toValue(options.active) || reducedMotion.value) return
    if (elapsed.value >= duration.value) elapsed.value = 0
    requested.value = true
  }
  function replay() {
    // Manual reduced-motion chapters display their completed, useful state.
    elapsed.value = reducedMotion.value ? interval.value * .92 : 0
    requested.value = toValue(options.active) && !reducedMotion.value
    schedule()
  }
  function toggle() { if (playing.value) pause(); else play() }
  function seek(value: number) {
    if (!Number.isFinite(value)) return
    pause()
    elapsed.value = Math.min(1, Math.max(0, value)) * duration.value
  }
  function selectStep(index: number) {
    if (!Number.isFinite(index)) return
    pause()
    elapsed.value = (Math.min(count.value - 1, Math.max(0, Math.trunc(index))) + .92) * interval.value
  }
  function visibilityChanged() { hidden.value = document.hidden }

  watch([() => toValue(options.active), count, interval, () => toValue(options.resetKey)], replay, { immediate: true })
  watch(reducedMotion, value => { if (value) { pause(); selectStep(step.value) } })
  watch(playing, schedule, { immediate: true, flush: 'sync' })

  onMounted(() => document.addEventListener('visibilitychange', visibilityChanged))
  onScopeDispose(() => { clear(); document.removeEventListener('visibilitychange', visibilityChanged) })
  return { step, phase, progress, elapsed: computed(() => elapsed.value), duration, playing, reducedMotion, seek, selectStep, play, pause, toggle, replay }
}
