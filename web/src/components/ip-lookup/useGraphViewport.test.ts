import { describe, expect, it, vi } from 'vitest'
import { fitGraph, useGraphViewport } from './useGraphViewport'

describe('graph fit and anchored gestures', () => {
  it('fits every edge of tall and wide views without a minimum-scale crop', () => {
    for (const [width, height] of [[700, 15000], [65000, 300]]) {
      const view = fitGraph(width!, height!, 320, 340)
      expect(view.offset.x).toBeGreaterThanOrEqual(15)
      expect(view.offset.y).toBeGreaterThanOrEqual(15)
      expect(view.offset.x + width! * view.scale).toBeLessThanOrEqual(305)
      expect(view.offset.y + height! * view.scale).toBeLessThanOrEqual(325)
    }
  })
  it('keeps the same graph point under the wheel anchor', () => {
    const state = useGraphViewport()
    state.offset.value = { x: 30, y: 20 }
    const anchor = { x: 160, y: 170 }
    state.zoomAt(2, anchor)
    expect(state.scale.value).toBe(2)
    expect(state.offset.value).toEqual({ x: -100, y: -130 })
  })
  it('pinches around two pointers and cancels cleanly', () => {
    const state = useGraphViewport()
    const target = { getBoundingClientRect: () => ({ left: 0, top: 0 }), setPointerCapture: vi.fn(), hasPointerCapture: () => false, closest: () => null }
    const event = (id: number, x: number) => ({ pointerId: id, pointerType: 'touch', clientX: x, clientY: 100, currentTarget: target, target }) as unknown as PointerEvent
    state.pointerDown(event(1, 100)); state.pointerDown(event(2, 200))
    state.pointerMove(event(2, 300))
    expect(state.scale.value).toBe(2)
    expect(state.offset.value).toEqual({ x: -100, y: -100 })
    state.pointerUp(event(2, 300)); state.pointerUp(event(1, 100))
    state.pointerMove(event(1, 150))
    expect(state.scale.value).toBe(2)
  })
})
