import { effectScope, ref, shallowRef } from 'vue'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { FeaturePaymentOrder } from '@/api/features'
import { usePaymentConfirmation } from './usePaymentConfirmation'

const motion = ref(false)
vi.mock('./useMotionPreferences', () => ({ useMotionPreferences: () => ({ reducedMotion: motion }) }))
vi.mock('@/utils/telegram', () => ({ notifyHaptic: vi.fn() }))

// This composable observes only the identity and authoritative status.
const receipt = (status: 'pending' | 'paid') => ({ id: 'credit', status }) as FeaturePaymentOrder

describe('wallet confirmation', () => {
  afterEach(() => { vi.restoreAllMocks(); vi.unstubAllGlobals(); motion.value = false })

  it('requires a newly paid order and advances only in the foreground', () => {
    let frame: FrameRequestCallback | undefined
    const focus = vi.spyOn(document, 'hasFocus').mockReturnValue(false)
    vi.spyOn(document, 'visibilityState', 'get').mockReturnValue('visible')
    vi.stubGlobal('requestAnimationFrame', (fn: FrameRequestCallback) => { frame = fn; return 1 })
    vi.stubGlobal('cancelAnimationFrame', () => { frame = undefined })
    const scope = effectScope(), order = shallowRef(receipt('pending'))
    try {
      const feedback = scope.run(() => usePaymentConfirmation(order))!
      feedback.confirm()
      expect(feedback.phase.value).toBe('idle')
      order.value = receipt('paid'); feedback.confirm()
      expect(feedback.phase.value).toBe('waiting')
      expect(frame).toBeUndefined()
      focus.mockReturnValue(true); window.dispatchEvent(new Event('focus'))
      frame!(0); frame!(600)
      expect(feedback.phase.value).toBe('coins')
      focus.mockReturnValue(false); window.dispatchEvent(new Event('blur'))
      expect(frame).toBeUndefined()
      focus.mockReturnValue(true); window.dispatchEvent(new Event('focus'))
      frame!(5000)
      expect(feedback.progress.value).toBe(0.4)
      frame!(5450); expect(feedback.phase.value).toBe('check')
      frame!(5900); expect(feedback.phase.value).toBe('complete')
      feedback.reset(); feedback.confirm()
      expect(feedback.phase.value).toBe('complete')
    } finally { scope.stop() }
  })

  it('shows an already-paid receipt without starting a new animation', () => {
    const scope = effectScope()
    try {
      const feedback = scope.run(() => usePaymentConfirmation(shallowRef(receipt('paid'))))!
      feedback.confirm()
      expect(feedback.phase.value).toBe('complete')
      expect(feedback.progress.value).toBe(1)
    } finally { scope.stop() }
  })
})
