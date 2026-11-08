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
    state.setInteraction(true)
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
  it('preserves ordinary wheel scrolling and browser pinch zoom until explicitly opted in', () => {
    const state = useGraphViewport()
    const target = document.createElement('div')
    target.addEventListener('wheel', state.wheel)
    const wheel = (modifier?: 'ctrlKey' | 'metaKey') => {
      const event = new WheelEvent('wheel', { deltaY: 50, cancelable: true })
      // Happy DOM's WheelEvent extends UIEvent, omitting inherited MouseEvent fields.
      Object.defineProperties(event, { clientX: { value: 20 }, clientY: { value: 30 },
        ctrlKey: { value: modifier === 'ctrlKey' }, metaKey: { value: modifier === 'metaKey' } })
      return event
    }
    const ordinary = wheel()
    target.dispatchEvent(ordinary)
    expect(ordinary.defaultPrevented).toBe(false)
    expect(state.scale.value).toBe(1)
    state.setInteraction(true)
    for (const modifier of ['ctrlKey', 'metaKey'] as const) {
      const browserZoom = wheel(modifier)
      expect(browserZoom[modifier]).toBe(true)
      target.dispatchEvent(browserZoom)
      expect(browserZoom.defaultPrevented).toBe(false)
      expect(state.scale.value).toBe(1)
    }
    const intentional = wheel()
    target.dispatchEvent(intentional)
    expect(intentional.defaultPrevented).toBe(true)
    expect(state.scale.value).toBeLessThan(1)
    expect((20 - state.offset.value.x) / state.scale.value).toBeCloseTo(20)
    expect((30 - state.offset.value.y) / state.scale.value).toBeCloseTo(30)
  })
  it('leaves touch scrolling native while preserving mouse panning', () => {
    const state = useGraphViewport()
    const target = { getBoundingClientRect: () => ({ left: 0, top: 0 }), setPointerCapture: vi.fn(), closest: () => null }
    const event = (pointerType: string, x: number) => ({ pointerId: 1, pointerType, button: 0, clientX: x, clientY: 0, currentTarget: target, target }) as unknown as PointerEvent
    state.pointerDown(event('touch', 0)); state.pointerMove(event('touch', 30))
    expect(state.offset.value).toEqual({ x: 0, y: 0 })
    expect(target.setPointerCapture).not.toHaveBeenCalled()
    state.pointerDown(event('mouse', 0)); state.pointerMove(event('mouse', 30))
    expect(state.offset.value).toEqual({ x: 30, y: 0 })
    expect(target.setPointerCapture).toHaveBeenCalledOnce()
  })
  it('releases captured pointers on Escape and ignores their later movements', () => {
    const state = useGraphViewport()
    const held = new Set<number>()
    const target = { getBoundingClientRect: () => ({ left: 0, top: 0 }), closest: () => null,
      setPointerCapture: (id: number) => held.add(id), hasPointerCapture: (id: number) => held.has(id),
      releasePointerCapture: (id: number) => held.delete(id) }
    const pointer = (id: number, x: number) => ({ pointerId: id, pointerType: 'touch', clientX: x, clientY: 0, currentTarget: target, target }) as unknown as PointerEvent
    state.setInteraction(true)
    state.pointerDown(pointer(1, 0)); state.pointerDown(pointer(2, 100))
    const event = { key: 'Escape', target, currentTarget: target, preventDefault: vi.fn(), stopPropagation: vi.fn() } as unknown as KeyboardEvent
    state.keyboard(event)
    expect(state.interactionEnabled.value).toBe(false)
    expect(held.size).toBe(0)
    expect(event.preventDefault).toHaveBeenCalledOnce()
    state.pointerMove(pointer(2, 200))
    expect(state.scale.value).toBe(1)
    expect(state.offset.value).toEqual({ x: 0, y: 0 })
  })
})
