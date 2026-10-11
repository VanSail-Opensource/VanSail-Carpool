import { shallowRef } from 'vue'

type Point = { x: number; y: number }
export function fitGraph(width: number, height: number, viewportWidth: number, viewportHeight: number) {
  const scale = Math.min(1, Math.max(1, viewportWidth - 32) / width, Math.max(1, viewportHeight - 32) / height)
  return { scale, offset: { x: (viewportWidth - width * scale) / 2, y: (viewportHeight - height * scale) / 2 } }
}

export function useGraphViewport() {
  const scale = shallowRef(1)
  const offset = shallowRef<Point>({ x: 0, y: 0 })
  const interacted = shallowRef(false)
  const pointers = new Map<number, Point>()
  let gesture: { center: Point; distance: number; scale: number; offset: Point; drag: boolean } | null = null
  let moved = false
  const geometry = () => {
    const points = [...pointers.values()]
    const a = points[0]!, b = points[1] ?? a
    return { center: { x: (a.x + b.x) / 2, y: (a.y + b.y) / 2 }, distance: Math.hypot(a.x - b.x, a.y - b.y) }
  }
  const local = (event: PointerEvent | WheelEvent) => {
    const rect = (event.currentTarget as HTMLElement).getBoundingClientRect()
    return { x: event.clientX - rect.left, y: event.clientY - rect.top }
  }
  function zoomAt(factor: number, anchor: Point) {
    const next = Math.max(0.001, Math.min(8, scale.value * factor))
    const ratio = next / scale.value
    offset.value = { x: anchor.x - (anchor.x - offset.value.x) * ratio, y: anchor.y - (anchor.y - offset.value.y) * ratio }
    scale.value = next
    interacted.value = true
  }
  function fit(width: number, height: number, viewWidth: number, viewHeight: number) {
    const view = fitGraph(width, height, viewWidth, viewHeight)
    scale.value = view.scale; offset.value = view.offset
  }
  function wheel(event: WheelEvent) {
    event.preventDefault()
    const delta = event.deltaY * (event.deltaMode === 1 ? 16 : event.deltaMode === 2 ? 300 : 1)
    zoomAt(Math.exp(-Math.max(-300, Math.min(300, delta)) * 0.003), local(event))
  }
  function pointerDown(event: PointerEvent) {
    if (event.pointerType === 'mouse' && event.button !== 0) return
    pointers.set(event.pointerId, local(event))
    const drag = !(event.target as Element).closest('[data-asn]')
    gesture = { ...geometry(), scale: scale.value, offset: offset.value, drag: drag || pointers.size > 1 }
    moved = false
    if (gesture.drag) (event.currentTarget as HTMLElement).setPointerCapture(event.pointerId)
  }
  function pointerMove(event: PointerEvent) {
    if (!pointers.has(event.pointerId) || !gesture) return
    pointers.set(event.pointerId, local(event))
    const next = geometry()
    const dx = next.center.x - gesture.center.x, dy = next.center.y - gesture.center.y
    if (!gesture.drag || (pointers.size === 1 && !moved && Math.hypot(dx, dy) < 4)) return
    const nextScale = pointers.size > 1 && gesture.distance > 0 ? Math.max(0.001, Math.min(8, gesture.scale * next.distance / gesture.distance)) : gesture.scale
    const ratio = nextScale / gesture.scale
    offset.value = { x: next.center.x - (gesture.center.x - gesture.offset.x) * ratio, y: next.center.y - (gesture.center.y - gesture.offset.y) * ratio }
    scale.value = nextScale
    interacted.value = moved = true
  }
  function pointerUp(event: PointerEvent) {
    pointers.delete(event.pointerId)
    const target = event.currentTarget as HTMLElement
    if (target.hasPointerCapture(event.pointerId)) target.releasePointerCapture(event.pointerId)
    gesture = pointers.size ? { ...geometry(), scale: scale.value, offset: offset.value, drag: true } : null
  }
  function keyboard(event: KeyboardEvent) {
    if ((event.target as Element).closest('[data-asn]')) return
    const arrows: Record<string, [number, number]> = { ArrowLeft: [30, 0], ArrowRight: [-30, 0], ArrowUp: [0, 30], ArrowDown: [0, -30] }
    if (arrows[event.key]) { event.preventDefault(); const [x, y] = arrows[event.key]!; offset.value = { x: offset.value.x + x, y: offset.value.y + y }; interacted.value = true }
    const target = event.currentTarget as HTMLElement
    if (['+', '=', '-'].includes(event.key)) { event.preventDefault(); zoomAt(event.key === '-' ? 1 / 1.2 : 1.2, { x: target.clientWidth / 2, y: target.clientHeight / 2 }) }
  }
  return { scale, offset, interacted, fit, zoomAt, wheel, pointerDown, pointerMove, pointerUp, keyboard, wasDragged: () => moved }
}
