interface Rectangle { left: number; top: number; right: number; bottom: number }

// Merge overlapping cutouts so even-odd clipping never paints their intersection.
export function modeTransitionMask(width: number, height: number, controls: readonly Rectangle[]): string {
  const holes: Rectangle[] = []
  for (const control of controls) {
    const rectangle = {
      left: Math.max(0, control.left - 8), top: Math.max(0, control.top - 8),
      right: Math.min(width, control.right + 8), bottom: Math.min(height, control.bottom + 8),
    }
    if (rectangle.right <= rectangle.left || rectangle.bottom <= rectangle.top) continue
    for (let index = 0; index < holes.length;) {
      const hole = holes[index]!
      if (rectangle.left > hole.right || rectangle.right < hole.left || rectangle.top > hole.bottom || rectangle.bottom < hole.top) {
        index++
        continue
      }
      rectangle.left = Math.min(rectangle.left, hole.left)
      rectangle.top = Math.min(rectangle.top, hole.top)
      rectangle.right = Math.max(rectangle.right, hole.right)
      rectangle.bottom = Math.max(rectangle.bottom, hole.bottom)
      holes.splice(index, 1)
      index = 0
    }
    holes.push(rectangle)
  }
  const box = (r: Rectangle) => `M${r.left} ${r.top}H${r.right}V${r.bottom}H${r.left}Z`
  return `path(evenodd, "${box({ left: 0, top: 0, right: width, bottom: height })}${holes.map(box).join('')}")`
}
