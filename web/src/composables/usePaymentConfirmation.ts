import { computed, onScopeDispose, readonly, shallowRef, watch, type Ref } from 'vue'

import type { FeaturePaymentOrder } from '@/api/features'
import { notifyHaptic } from '@/utils/telegram'
import { useMotionPreferences } from './useMotionPreferences'

export type PaymentConfirmationPhase = 'idle' | 'waiting' | 'coins' | 'check' | 'complete'

export function usePaymentConfirmation(order: Readonly<Ref<FeaturePaymentOrder | null>>) {
  const { reducedMotion } = useMotionPreferences()
  const phase = shallowRef<PaymentConfirmationPhase>('idle')
  const elapsed = shallowRef(0)
  const progress = computed(() => Math.min(1, elapsed.value / 1500))
  const presented = new Set<string>()
  let pendingId: string | null = null
  let frame: number | undefined
  let previousFrame: number | undefined
  let notified = false

  function stopFrame(): void {
    if (frame !== undefined) cancelAnimationFrame(frame)
    frame = undefined
    previousFrame = undefined
  }

  function reset(): void {
    stopFrame()
    pendingId = null
    phase.value = 'idle'
    elapsed.value = 0
    notified = false
  }

  function foreground(): boolean {
    return document.visibilityState === 'visible' && document.hasFocus()
  }

  function tick(now: number): void {
    frame = undefined
    if (!foreground()) { stopFrame(); return }
    if (previousFrame !== undefined) elapsed.value += Math.max(0, now - previousFrame)
    previousFrame = now
    phase.value = elapsed.value < 1050 ? 'coins' : elapsed.value < 1500 ? 'check' : 'complete'
    if (phase.value !== 'coins' && !notified) {
      notified = true
      notifyHaptic('success')
    }
    if (phase.value !== 'complete') frame = requestAnimationFrame(tick)
    else stopFrame()
  }

  function resume(): void {
    if (!foreground()) { stopFrame(); return }
    if (!['waiting', 'coins', 'check'].includes(phase.value)) return
    if (reducedMotion.value) {
      stopFrame()
      elapsed.value = 1500
      phase.value = 'complete'
      return
    }
    if (frame === undefined) frame = requestAnimationFrame(tick)
  }

  // Remember a pending baseline; only confirm() called by the server-paid path
  // may start feedback. A receipt success, countdown or copy action cannot do so.
  watch(() => [order.value?.id, order.value?.status] as const, ([id, status], previous) => {
    if (id !== previous?.[0] || (status !== 'pending' && status !== 'paid')) reset()
    if (status === 'pending' && id && !presented.has(id)) pendingId = id
    else if (status !== 'paid') pendingId = null
  }, { flush: 'sync', immediate: true })

  function confirm(): void {
    const current = order.value
    if (!current || current.status !== 'paid'
      || !['builtin_usdt', 'bepusdt'].includes(current.provider)) return
    if (presented.has(current.id)) {
      if (phase.value === 'idle') { elapsed.value = 1500; phase.value = 'complete' }
      return
    }
    presented.add(current.id)
    const newlyConfirmed = pendingId === current.id
    stopFrame()
    elapsed.value = newlyConfirmed ? 0 : 1500
    phase.value = newlyConfirmed ? 'waiting' : 'complete'
    notified = false
    resume()
  }

  document.addEventListener('visibilitychange', resume)
  window.addEventListener('focus', resume)
  window.addEventListener('blur', stopFrame)
  watch(reducedMotion, resume, { flush: 'sync' })
  onScopeDispose(() => {
    reset()
    document.removeEventListener('visibilitychange', resume)
    window.removeEventListener('focus', resume)
    window.removeEventListener('blur', stopFrame)
  })
  return { phase: readonly(phase), progress, confirm, reset }
}
