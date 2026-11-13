import { onMounted, onScopeDispose, readonly, shallowRef, watch } from 'vue'

import type { ActivityResult, LuckyDraw } from '@/api/features'
import { useMotionPreferences } from '@/composables/useMotionPreferences'
import { localizedError } from '@/i18n'
import { resolveDrawStyle, type DrawPhase, type DrawStyle, type DrawStyleChoice } from './selection'

export function useDrawExperience(play: (drawId: string) => Promise<ActivityResult>) {
  const draw = shallowRef<LuckyDraw | null>(null)
  const style = shallowRef<DrawStyle>('simple')
  const phase = shallowRef<DrawPhase>('running')
  const result = shallowRef<ActivityResult | null>(null)
  const error = shallowRef<string | null>(null)
  const resetToken = shallowRef(0)
  const started = shallowRef(false)
  const { reducedMotion } = useMotionPreferences()
  let watchdog: ReturnType<typeof setTimeout> | undefined
  let disposed = false
  const requiresStart = () => style.value === 'grid' || style.value === 'wheel' || style.value === 'slot'

  function armWatchdog(): void {
    if (watchdog) clearTimeout(watchdog)
    if (phase.value !== 'settling' || (requiresStart() && !started.value)) return
    watchdog = setTimeout(reveal, style.value === 'wheel' ? 11000 : 6000)
  }

  function markStarted(): void {
    started.value = true
    armWatchdog()
  }

  function reveal(): void {
    if (!result.value) return
    phase.value = 'receipt'
  }

  async function submit(): Promise<void> {
    const current = draw.value
    if (disposed || !current) return
    phase.value = 'running'
    error.value = null
    try {
      const received = await play(current.id)
      if (disposed || draw.value?.id !== current.id) return
      result.value = received
      phase.value = (reducedMotion.value && !requiresStart()) || globalThis.document?.hidden || !received.prizeId
        ? 'receipt' : 'settling'
    } catch (caught) {
      if (disposed || draw.value?.id !== current.id) return
      error.value = localizedError(caught, 'errors.activityFailed')
      phase.value = 'error'
    }
  }

  function begin(selectedDraw: LuckyDraw, choice: DrawStyleChoice): void {
    if (disposed || draw.value) return
    draw.value = selectedDraw
    style.value = resolveDrawStyle(choice)
    started.value = false
    result.value = null
    void submit()
  }

  function retry(): void {
    if (phase.value === 'error' && draw.value) {
      started.value = false
      void submit()
    }
  }

  function close(): void {
    if (phase.value !== 'receipt' && phase.value !== 'error') return
    draw.value = null
    result.value = null
    error.value = null
    resetToken.value += 1
  }

  function handleVisibility(): void {
    if (globalThis.document?.hidden && result.value) reveal()
  }

  watch(phase, armWatchdog)
  watch(reducedMotion, (value) => {
    if (value && result.value && (!requiresStart() || started.value)) reveal()
  })
  onMounted(() => globalThis.document?.addEventListener('visibilitychange', handleVisibility))
  onScopeDispose(() => {
    disposed = true
    draw.value = null
    result.value = null
    if (watchdog) clearTimeout(watchdog)
    globalThis.document?.removeEventListener('visibilitychange', handleVisibility)
  })

  return {
    draw: readonly(draw), style: readonly(style), phase: readonly(phase), started: readonly(started),
    result: readonly(result), error: readonly(error), resetToken: readonly(resetToken),
    begin, retry, reveal, close, markStarted,
  }
}
