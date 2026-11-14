import { between, ease } from '@/components/common/illustrationTimeline'

export type BillingDemoMode = 'traffic' | 'hourly'

export const billingDemoSteps = {
  traffic: [
    { id: 'fund', icon: 'i-ph-wallet-bold', points: ['i-ph-wallet-bold', 'i-ph-arrows-down-up-bold', 'i-ph-hard-drives-bold'] },
    { id: 'connect', icon: 'i-ph-plugs-connected-bold', points: ['i-ph-hard-drives-bold', 'i-ph-play-circle-bold', 'i-ph-wallet-bold'] },
    { id: 'firstBlock', icon: 'i-ph-cube-bold', points: ['i-ph-gauge-bold', 'i-ph-cube-bold', 'i-ph-stack-bold'] },
    { id: 'useCredit', icon: 'i-ph-arrows-down-up-bold', points: ['i-ph-stack-bold', 'i-ph-chart-pie-bold', 'i-ph-wallet-bold'] },
    { id: 'nextBlock', icon: 'i-ph-plus-circle-bold', points: ['i-ph-arrows-down-up-bold', 'i-ph-plus-circle-bold', 'i-ph-coins-bold'] },
    { id: 'reconnect', icon: 'i-ph-arrows-clockwise-bold', points: ['i-ph-hard-drives-bold', 'i-ph-stack-bold', 'i-ph-shield-check-bold'] },
    { id: 'perNode', icon: 'i-ph-hard-drives-bold', points: ['i-ph-gauge-bold', 'i-ph-hard-drives-bold', 'i-ph-tag-bold'] },
    { id: 'expire', icon: 'i-ph-hourglass-bold', points: ['i-ph-hourglass-bold', 'i-ph-pause-circle-bold', 'i-ph-wallet-bold'] },
    { id: 'review', icon: 'i-ph-chart-line-bold', points: ['i-ph-hard-drives-bold', 'i-ph-chart-bar-bold', 'i-ph-magnifying-glass-bold'] },
  ],
  hourly: [
    { id: 'eligibility', icon: 'i-ph-wallet-bold', points: ['i-ph-wallet-bold', 'i-ph-coins-bold', 'i-ph-lock-open-bold'] },
    { id: 'enable', icon: 'i-ph-play-circle-bold', points: ['i-ph-play-circle-bold', 'i-ph-clock-bold', 'i-ph-coins-bold'] },
    { id: 'renew', icon: 'i-ph-clock-clockwise-bold', points: ['i-ph-moon-bold', 'i-ph-hard-drives-bold', 'i-ph-stop-circle-bold'] },
    { id: 'pool', icon: 'i-ph-circles-three-bold', points: ['i-ph-users-three-bold', 'i-ph-lightning-bold', 'i-ph-shield-check-bold'] },
    { id: 'throttle', icon: 'i-ph-gauge-bold', points: ['i-ph-gauge-bold', 'i-ph-toggle-left-bold', 'i-ph-clock-bold'] },
    { id: 'consent', icon: 'i-ph-toggle-right-bold', points: ['i-ph-tag-bold', 'i-ph-check-circle-bold', 'i-ph-clock-countdown-bold'] },
    { id: 'extra', icon: 'i-ph-rocket-launch-bold', points: ['i-ph-cube-bold', 'i-ph-coins-bold', 'i-ph-gauge-bold'] },
    { id: 'optOut', icon: 'i-ph-shield-check-bold', points: ['i-ph-toggle-left-bold', 'i-ph-prohibit-bold', 'i-ph-stack-bold'] },
    { id: 'disable', icon: 'i-ph-stop-circle-bold', points: ['i-ph-coins-bold', 'i-ph-hourglass-bold', 'i-ph-stack-bold'] },
    { id: 'midnight', icon: 'i-ph-moon-stars-bold', points: ['i-ph-calendar-check-bold', 'i-ph-hourglass-bold', 'i-ph-stop-circle-bold'] },
  ],
} as const

// Illustrative snapshots only: traffic uses whole binary-GB blocks; hourly
// extras use decimal GB. No values enter account state, quotes or APIs.
export const trafficDemoFrames = [
  { usageMiB: 0 }, { usageMiB: 0 }, { usageMiB: 100 },
  { usageMiB: 512 }, { usageMiB: 1536 }, { usageMiB: 1536 },
  { usageMiB: 1536 }, { usageMiB: 1536 }, { usageMiB: 1536 },
] as const

export const hourlyDemoFrames = [
  { time: '12:30', balance: '10.00', debit: '0.00', remaining: 100 },
  { time: '12:30', balance: '9.50', debit: '0.50', remaining: 100 },
  { time: '13:00', balance: '8.50', debit: '1.00', remaining: 100 },
  { time: '13:10', balance: '8.50', debit: '0.00', remaining: 50 },
  { time: '13:20', balance: '8.50', debit: '0.00', remaining: 0 },
  { time: '13:20', balance: '8.50', debit: '0.00', remaining: 0 },
  { time: '13:25', balance: '8.30', debit: '0.20', remaining: 100 },
  { time: '13:30', balance: '8.30', debit: '0.00', remaining: 60 },
  { time: '13:40', balance: '8.30', debit: '0.00', remaining: 60 },
  { time: '00:00', balance: '8.30', debit: '0.00', remaining: 100 },
] as const

/** Usage moves continuously; the wallet changes only at block boundaries. */
export function trafficDemoState(step: number, phase: number) {
  const current = trafficDemoFrames[step] ?? trafficDemoFrames[0]
  const previous = trafficDemoFrames[Math.max(0, step - 1)] ?? trafficDemoFrames[0]
  const usageMiB = Math.round(previous.usageMiB + (current.usageMiB - previous.usageMiB) * ease(between(phase, .12, .7)))
  const blocks = usageMiB < 100 ? 0 : Math.max(1, Math.ceil(usageMiB / 1024))
  const expired = step > 7 || (step === 7 && phase >= .62)
  const funded = step > 0 || phase >= .45
  return {
    usageMiB, blocks, expired, funded,
    creditMiB: expired ? 0 : Math.max(0, blocks * 1024 - usageMiB),
    balance: (funded ? 10 - blocks : 0).toFixed(2),
    connected: step === 0 ? false : step === 1 ? phase >= .3 : step === 5 ? phase >= .4 : true,
  }
}

/** Eligibility, consent and funding are three separate observable states. */
export function hourlyDemoState(step: number, phase: number) {
  const current = hourlyDemoFrames[step] ?? hourlyDemoFrames[0]
  const previous = hourlyDemoFrames[Math.max(0, step - 1)] ?? hourlyDemoFrames[0]
  const actionAt = step === 2 ? .62 : step === 6 ? .52 : .32
  const acted = phase >= actionAt
  const reset = step === 9 && phase >= .55
  const extra = (step > 6 || (step === 6 && acted)) && !reset
  let remaining: number = current.remaining
  if (step === 3 || step === 4) remaining = previous.remaining + (current.remaining - previous.remaining) * ease(between(phase, .08, .7))
  if (step === 6 && !acted) remaining = 0
  if (step === 7) remaining = 100 - 40 * ease(between(phase, .04, .3))
  if (step === 9 && !reset) remaining = 60
  const charging = [1, 2, 6].includes(step)
  const balance = charging && !acted ? previous.balance : current.balance
  const enabled = step === 0 ? false : step === 1 ? acted : step < 8 || (step === 8 && !acted)
  const autoBuy = (step === 5 && phase >= .3) || step === 6 || (step === 7 && phase < .32)
  return {
    remaining, balance, enabled, autoBuy, extra, reset,
    debit: charging && acted ? current.debit : '0.00',
    time: step === 2 && !acted ? '12:59' : step === 9 && !reset ? '23:59' : current.time,
    reduced: enabled && remaining <= 0,
  }
}
