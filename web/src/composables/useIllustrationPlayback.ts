import { computed, onMounted, onScopeDispose, readonly, shallowRef, toValue, watch, type MaybeRefOrGetter } from 'vue'
import { useMotionPreferences } from './useMotionPreferences'

/** Local, read-only walkthrough playback. It never starts work on the server. */
export function useIllustrationPlayback(options: {
  active: MaybeRefOrGetter<boolean>
  stepCount: MaybeRefOrGetter<number>
  resetKey?: MaybeRefOrGetter<unknown>
  interval?: number
}) {
  const { reducedMotion } = useMotionPreferences()
  const step = shallowRef(0)
  const requested = shallowRef(false)
  const hidden = shallowRef(typeof document !== 'undefined' && document.hidden)
  const count = computed(() => Math.max(1, toValue(options.stepCount)))
  const playing = computed(() => toValue(options.active) && requested.value && !hidden.value && !reducedMotion.value)
  let timer: ReturnType<typeof setTimeout> | undefined

  function clear() { if (timer !== undefined) clearTimeout(timer); timer = undefined }
  function pause() { requested.value = false }
  function play() {
    if (!toValue(options.active) || reducedMotion.value) return
    if (step.value === count.value - 1) step.value = 0
    requested.value = true
  }
  function replay() {
    step.value = 0
    requested.value = toValue(options.active) && !reducedMotion.value
    // Replaying step zero can leave both refs unchanged; restart its full
    // interval explicitly instead of relying only on the reactive watcher.
    schedule()
  }
  function toggle() { if (playing.value) pause(); else play() }
  function selectStep(index: number) {
    if (!Number.isFinite(index)) return
    pause()
    step.value = Math.min(count.value - 1, Math.max(0, Math.trunc(index)))
  }
  function visibilityChanged() { hidden.value = document.hidden }

  watch([() => toValue(options.active), count, () => toValue(options.resetKey)], replay, { immediate: true })
  watch(reducedMotion, value => { if (value) pause() })
  function schedule() {
    clear()
    if (!playing.value) return
    timer = setTimeout(() => {
      timer = undefined
      if (step.value < count.value - 1) step.value++
      else pause()
    }, options.interval ?? 2600)
  }
  watch([playing, step], schedule, { immediate: true })

  onMounted(() => document.addEventListener('visibilitychange', visibilityChanged))
  onScopeDispose(() => { clear(); document.removeEventListener('visibilitychange', visibilityChanged) })
  return { step: readonly(step), playing, reducedMotion, selectStep, play, pause, toggle, replay }
}
