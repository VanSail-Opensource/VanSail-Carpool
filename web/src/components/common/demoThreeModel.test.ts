import { describe, expect, it } from 'vitest'
import { demoThreeActive, demoThreeCapacityScale, demoThreeRouteState, demoThreeStory, type DemoThreeFrame } from './demoThreeModel'

const frame = (topic: DemoThreeFrame['topic'], step: number, phase = .8): DemoThreeFrame => ({ topic, step, phase, reducedMotion: false })
const flows = (state: DemoThreeFrame) => demoThreeStory(state.topic).routes.filter(route => demoThreeRouteState(state, route).flowing).map(route => `${route.from}:${route.to}`)
const charged = (state: DemoThreeFrame) => flows(state).some(route => route.startsWith('wallet:') || route.endsWith(':wallet'))

describe('tutorial route semantics', () => {
  it('shows one final exit at a time instead of duplicating traffic', () => {
    const international = frame('networks', 3, .25), broadband = frame('networks', 3, .75)
    expect(flows(international)).toEqual(['you:entry', 'entry:international', 'international:service'])
    expect(flows(broadband)).toEqual(['you:entry', 'entry:broadband', 'broadband:service'])
    expect(demoThreeActive(international)).not.toContain('broadband')
    expect(demoThreeActive(broadband)).not.toContain('international')
    for (const phase of [.1, .9]) expect(flows({ ...frame('networks', 3, phase), reducedMotion: true })).toEqual(flows(broadband))
  })

  it('highlights configuration without sending data before routing is applied', () => {
    for (const step of [0, 1, 2]) {
      for (const phase of [0, .5, 1]) expect(flows(frame('routing', step, phase))).toEqual([])
      expect(demoThreeActive(frame('routing', step))).toContain('entry')
    }
    expect(flows({ ...frame('routing', 3), matches: true })).toEqual(['you:entry', 'entry:match', 'match:service'])
    expect(flows({ ...frame('routing', 3), matches: false })).toEqual(['you:entry', 'entry:default', 'default:service'])
  })

  it('keeps traffic purchase pulses behind the first and next block thresholds', () => {
    expect(charged(frame('traffic', 1, .4))).toBe(false)
    expect(charged(frame('traffic', 1, .7))).toBe(true)
    expect(charged(frame('traffic', 3, .4))).toBe(false)
    expect(charged(frame('traffic', 3, .45))).toBe(true)
    // A completed charge is held visibly, never represented as an endless debit loop.
    expect(charged(frame('traffic', 1, 1))).toBe(false)
    for (const step of [0, 2, 4, 5]) for (const phase of [0, .4, .8, 1]) expect(charged(frame('traffic', step, phase))).toBe(false)
    expect(flows(frame('traffic', 4, .2))).toEqual([])
    expect(flows(frame('traffic', 4, .8))).toEqual(['client:node'])
  })

  it('waits for hourly activation, renewal and consented extra charges', () => {
    expect(charged(frame('hourly', 0, .2))).toBe(false)
    expect(charged(frame('hourly', 0, .4))).toBe(true)
    expect(charged(frame('hourly', 1, .5))).toBe(false)
    expect(charged(frame('hourly', 1, .7))).toBe(true)
    // The extra toggle has been enabled at .4, but the purchase has not happened yet.
    expect(charged(frame('hourly', 4, .4))).toBe(false)
    expect(flows(frame('hourly', 4, .6))).toContain('wallet:extra')
    expect(flows(frame('hourly', 4, .6))).toContain('extra:pool')
  })

  it('does not show charges or traffic after disable or the midnight reset', () => {
    for (const step of [5, 6]) for (const phase of [.4, .8, 1]) expect(flows(frame('hourly', step, phase))).toEqual([])
    for (const step of [2, 3, 5, 6]) for (const phase of [0, .4, .8, 1]) expect(charged(frame('hourly', step, phase))).toBe(false)
  })

  it('keeps capacity visible until expiry and restores extras only after purchase', () => {
    expect(demoThreeCapacityScale(frame('traffic', 5, .5), 'credit')).toBe(1)
    expect(demoThreeCapacityScale(frame('traffic', 5, .8), 'credit')).toBe(.35)
    expect(demoThreeCapacityScale(frame('hourly', 3, 1), 'pool')).toBe(.28)
    expect(demoThreeCapacityScale(frame('hourly', 4, .4), 'pool')).toBe(.28)
    expect(demoThreeCapacityScale(frame('hourly', 4, .6), 'pool')).toBe(1)
  })
})
