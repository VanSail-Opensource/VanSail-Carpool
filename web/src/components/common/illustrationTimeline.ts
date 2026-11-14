export function between(value: number, start = 0, end = 1): number {
  return Math.min(1, Math.max(0, (value - start) / (end - start)))
}

export function ease(value: number): number { return value * value * (3 - 2 * value) }

export interface DemoInteraction { at: number; x: number; y: number }

/** Coordinates are percentages of the persistent scene, not viewport pixels. */
export function interactionCue(phase: number, points: readonly DemoInteraction[]) {
  const current = points.findIndex(point => phase < point.at + .1)
  const target = points[current === -1 ? points.length - 1 : current]
  if (!target) return { x: 0, y: 0, press: 0, opacity: 0 }
  const previous = current > 0 ? points[current - 1]! : { at: -.12, x: 8, y: 92 }
  const movement = ease(between(phase, previous.at + .1, target.at))
  const pressPhase = between(phase, target.at, target.at + .1)
  return {
    x: previous.x + (target.x - previous.x) * movement,
    y: previous.y + (target.y - previous.y) * movement,
    press: pressPhase > 0 && pressPhase < 1 ? Math.sin(pressPhase * Math.PI) : 0,
    opacity: between(phase, 0, .08) * (1 - between(phase, points.at(-1)!.at + .12, points.at(-1)!.at + .28)),
  }
}
