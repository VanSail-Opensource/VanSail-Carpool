import { hourlyDemoState, trafficDemoState } from '@/components/catalog/billingDemo'

export type DemoThreeTopic = 'subscription' | 'networks' | 'payment' | 'sharing' | 'rules' | 'why' | 'hourly' | 'traffic' | 'routing'
export type DemoThreeShape = 'phone' | 'server' | 'globe' | 'home' | 'wallet' | 'coin' | 'invoice' | 'chat' | 'users' | 'shield' | 'clock' | 'cube' | 'code' | 'link' | 'download'
export interface DemoThreeFrame { topic: DemoThreeTopic; step: number; phase: number; reducedMotion: boolean; matches?: boolean }
export interface DemoThreeNode { id: string; shape: DemoThreeShape; x: number; y: number; scale?: number; warning?: boolean }
export interface DemoThreeRoute { from: string; to: string; bend?: number; warning?: boolean }
export interface DemoThreeStory { nodes: readonly DemoThreeNode[]; routes: readonly DemoThreeRoute[]; stages: readonly (readonly string[])[] }

const node = (id: string, shape: DemoThreeShape, x: number, y: number, scale = 1, warning = false): DemoThreeNode => ({ id, shape, x, y, scale, warning })
const route = (from: string, to: string, bend = 0, warning = false): DemoThreeRoute => ({ from, to, bend, warning })

/** Persistent objects carry the story between chapters; only their emphasis changes. */
const stories: Record<DemoThreeTopic, DemoThreeStory> = {
  networks: {
    nodes: [node('you', 'phone', -3.45, -.1), node('entry', 'server', -1.55, -.1), node('international', 'globe', .7, 1.15), node('broadband', 'home', .7, -1.1), node('service', 'globe', 3.2, -.1)],
    routes: [route('you', 'entry'), route('entry', 'international', .35), route('international', 'service', .35), route('entry', 'broadband', -.35), route('broadband', 'service', -.35)],
    stages: [['you', 'entry'], ['you', 'entry', 'international', 'service'], ['you', 'entry', 'broadband', 'service'], ['you', 'entry', 'international', 'broadband', 'service']],
  },
  payment: {
    nodes: [node('wallet', 'wallet', -3.25, .2), node('amount', 'coin', -1.1, .45), node('invoice', 'invoice', 1.25, .35), node('gas', 'globe', -.8, -1.35, .65), node('support', 'chat', 3.25, -.35)],
    routes: [route('wallet', 'amount', .25), route('amount', 'invoice', .25), route('wallet', 'gas', -.4), route('invoice', 'support', -.3)],
    stages: [['wallet', 'amount', 'invoice'], ['wallet', 'amount', 'invoice', 'gas'], ['wallet', 'amount', 'invoice'], ['invoice', 'support']],
  },
  sharing: {
    nodes: [node('members', 'users', -3.1, .25, 1.15), node('node', 'server', -.9, .25, 1.2), node('policy', 'shield', 1.25, -1.05, .8), node('service', 'globe', 3, .35), node('signals', 'phone', 1.15, 1.25, .65)],
    routes: [route('members', 'node'), route('node', 'service', .25), route('policy', 'node', -.25), route('signals', 'service', .2)],
    stages: [['members', 'node'], ['members', 'node', 'service'], ['members', 'node', 'policy'], ['node', 'policy', 'signals', 'service']],
  },
  rules: {
    nodes: [node('everyday', 'phone', -3.15, .85), node('abuse', 'code', -3.15, -1.15, .85, true), node('gate', 'shield', -.25, .1, 1.55), node('service', 'globe', 2.65, .8), node('support', 'chat', 2.65, -1.05, .8)],
    routes: [route('everyday', 'gate', .25), route('gate', 'service', .25), route('abuse', 'gate', -.25, true), route('gate', 'support', -.25)],
    stages: [['everyday', 'gate', 'service'], ['abuse', 'gate'], ['abuse', 'gate'], ['everyday', 'gate', 'support']],
  },
  why: {
    nodes: [node('billing', 'clock', -3.1, .35), node('code', 'code', -1.25, -.85), node('service', 'server', .55, .75, 1.2), node('privacy', 'shield', 2.5, -.85), node('support', 'chat', 3.2, 1)],
    routes: [route('billing', 'service', .55), route('code', 'service', -.3), route('service', 'privacy', -.3), route('service', 'support', .25)],
    stages: [['billing', 'service'], ['code', 'service'], ['service', 'support'], ['privacy', 'service', 'support']],
  },
  subscription: {
    nodes: [node('wallet', 'wallet', -3.2, 1), node('billing', 'clock', -3.1, -1.1, .85), node('routing', 'server', -.95, 1.5, .7), node('client', 'phone', 0, -.1, 1.45), node('download', 'download', 1.25, 1.45, .8), node('link', 'link', 2.85, -1.05, .9), node('node', 'globe', 3.3, .8)],
    routes: [route('wallet', 'client', .2), route('billing', 'client', -.4), route('routing', 'client', -.2), route('download', 'client', .2), route('link', 'client', -.25), route('client', 'node', .35)],
    stages: [['wallet', 'client'], ['billing', 'client'], ['routing', 'client'], ['download', 'client'], ['link', 'client'], ['client', 'node']],
  },
  hourly: {
    nodes: [node('squad', 'server', -3.15, .1), node('clock', 'clock', -1, .6, 1.15), node('pool', 'cube', 1.05, .1, 1.1), node('wallet', 'wallet', 3.15, .35), node('extra', 'coin', 1.3, -1.35, .65)],
    routes: [route('squad', 'clock', .4), route('clock', 'wallet', .6), route('squad', 'pool', -.5), route('wallet', 'extra', -.35), route('extra', 'pool')],
    stages: [['squad', 'clock', 'wallet'], ['squad', 'clock', 'wallet'], ['squad', 'pool'], ['squad', 'clock'], ['squad', 'pool', 'wallet', 'extra'], ['clock', 'pool'], ['pool', 'clock']],
  },
  traffic: {
    nodes: [node('client', 'phone', -3.2, .15), node('node', 'server', -1.2, .15), node('credit', 'cube', 1.15, .35, 1.25), node('wallet', 'wallet', 3.25, .15), node('expiry', 'clock', 1.15, -1.4, .6)],
    routes: [route('client', 'node'), route('node', 'credit', .35), route('wallet', 'credit', .4), route('credit', 'expiry')],
    stages: [['client', 'node'], ['client', 'node', 'credit', 'wallet'], ['client', 'node', 'credit'], ['node', 'credit', 'wallet'], ['client', 'node', 'credit'], ['credit', 'expiry']],
  },
  routing: {
    nodes: [node('you', 'phone', -3.5, 0), node('entry', 'server', -1.65, 0), node('match', 'server', .75, 1.2), node('default', 'globe', .75, -1.2, .85), node('service', 'globe', 3.25, 0)],
    routes: [route('you', 'entry'), route('entry', 'match', .4), route('match', 'service', .35), route('entry', 'default', -.4), route('default', 'service', -.35)],
    stages: [['entry', 'match'], ['entry', 'match'], ['entry', 'match'], ['you', 'entry', 'match', 'service']],
  },
}

export const demoThreeStory = (topic: DemoThreeTopic): DemoThreeStory => stories[topic]

export function demoThreeActive(frame: DemoThreeFrame): readonly string[] {
  const story = stories[frame.topic]
  const active = story.stages[Math.min(story.stages.length - 1, Math.max(0, frame.step))] ?? []
  if (frame.topic === 'networks' && frame.step === 3) return ['you', 'entry', frame.reducedMotion || frame.phase >= .5 ? 'broadband' : 'international', 'service']
  if (frame.topic === 'routing' && frame.step === 3 && frame.matches === false) return ['you', 'entry', 'default', 'service']
  return active
}

/** Emphasis explains a selection; moving packets represent an action that has happened. */
export function demoThreeRouteState(frame: DemoThreeFrame, route: DemoThreeRoute): { highlighted: boolean; flowing: boolean } {
  const active = demoThreeActive(frame)
  const highlighted = active.includes(route.from) && active.includes(route.to)
  const phase = frame.reducedMotion ? .92 : Math.min(1, Math.max(0, frame.phase))
  // Show a single short event pulse instead of suggesting continuous wallet debits.
  const previousPhase = Math.max(0, phase - .18)
  let flowing = highlighted
  if (frame.topic === 'routing') flowing &&= frame.step === 3
  if (frame.topic === 'traffic') {
    const state = trafficDemoState(frame.step, phase), previous = trafficDemoState(frame.step, previousPhase)
    if (route.from === 'wallet') flowing &&= state.blocks > previous.blocks
    else if (route.to === 'credit') flowing &&= state.blocks > 0 && state.usageMiB > previous.usageMiB
    else if (route.to === 'expiry') flowing &&= state.expired && !previous.expired
    else flowing &&= state.connected && frame.step !== 5
  }
  if (frame.topic === 'hourly') {
    const state = hourlyDemoState(frame.step, phase), previous = hourlyDemoState(frame.step, previousPhase)
    const purchase = state.debit !== '0.00' && previous.debit === '0.00'
    flowing &&= state.enabled
    if (route.to === 'wallet') flowing &&= frame.step < 2 && purchase
    else if (route.from === 'wallet' || route.from === 'extra') flowing &&= frame.step === 4 && state.autoBuy && purchase
    else if (route.to === 'pool') flowing &&= !state.reduced && state.remaining < previous.remaining
  }
  return { highlighted, flowing }
}

/** Capacity poses follow the same synthetic snapshots as the visible counters. */
export function demoThreeCapacityScale(frame: DemoThreeFrame, id: string): number {
  const phase = frame.reducedMotion ? .92 : Math.min(1, Math.max(0, frame.phase))
  if (frame.topic === 'traffic' && frame.step === 5 && id === 'credit') return trafficDemoState(frame.step, phase).expired ? .35 : 1
  if (frame.topic === 'hourly' && id === 'pool') return .28 + .72 * Math.cbrt(hourlyDemoState(frame.step, phase).remaining / 100)
  return 1
}

/** Icon paths also provide an immediate, useful fallback when WebGL is unavailable. */
export const demoThreeSymbols: Record<DemoThreeShape, string> = {
  phone: 'M7 2h10v20H7z M10 18h4 M9 6h6 M9 9h6',
  server: 'M3 3h18v5H3z M3 10h18v5H3z M3 17h18v5H3z M6 5h1 M6 12h1 M6 19h1',
  globe: 'M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0 M3 12h18 M12 3c-6 5-6 13 0 18 M12 3c6 5 6 13 0 18',
  home: 'M3 11l9-8 9 8 M5 10v11h14V10 M10 21v-7h4v7',
  wallet: 'M3 6h17v15H3z M3 6l14-3v3 M15 12h7v5h-7z M18 14h1',
  coin: 'M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0 M15 8h-5a2 2 0 0 0 0 4h4a2 2 0 0 1 0 4H9 M12 5v14',
  invoice: 'M5 2h14v20l-3-2-4 2-4-2-3 2z M8 7h8 M8 11h8 M8 15h5',
  chat: 'M3 3h18v14H10l-7 5z M7 8h10 M7 12h6',
  users: 'M8 7a3 3 0 1 1-6 0 3 3 0 0 1 6 0 M22 7a3 3 0 1 1-6 0 3 3 0 0 1 6 0 M15 11a3 3 0 1 1-6 0 3 3 0 0 1 6 0 M1 17v-3h6 M17 14h6v3 M7 22v-5h10v5',
  shield: 'M12 2l9 4v6c0 5-5 9-9 11-4-2-9-6-9-11V6z M7 12l3 3 7-7',
  clock: 'M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0 M12 6v6l4 2',
  cube: 'M12 2l10 5v10l-10 5-10-5V7z M2 7l10 5 10-5 M12 12v10',
  code: 'M8 6l-6 6 6 6 M16 6l6 6-6 6 M14 3l-4 18',
  link: 'M10 7l3-3a5 5 0 0 1 7 7l-3 3 M14 17l-3 3a5 5 0 0 1-7-7l3-3 M8 16l8-8',
  download: 'M12 2v14 M6 10l6 6 6-6 M3 17v5h18v-5',
}
