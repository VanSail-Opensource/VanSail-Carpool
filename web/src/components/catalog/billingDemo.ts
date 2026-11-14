import { between, ease } from '@/components/common/illustrationTimeline'

export type BillingDemoMode = 'traffic' | 'hourly'

export const billingDemoSteps = {
  traffic: [
    { id: 'connect', icon: 'i-ph-plugs-connected' },
    { id: 'firstBlock', icon: 'i-ph-cube' },
    { id: 'useCredit', icon: 'i-ph-arrows-down-up' },
    { id: 'nextBlock', icon: 'i-ph-plus-circle' },
    { id: 'reconnect', icon: 'i-ph-arrows-clockwise' },
    { id: 'expire', icon: 'i-ph-hourglass' },
  ],
  hourly: [
    { id: 'enable', icon: 'i-ph-play-circle' },
    { id: 'renew', icon: 'i-ph-clock-clockwise' },
    { id: 'pool', icon: 'i-ph-circles-three' },
    { id: 'throttle', icon: 'i-ph-gauge' },
    { id: 'extra', icon: 'i-ph-rocket-launch' },
    { id: 'disable', icon: 'i-ph-stop-circle' },
    { id: 'midnight', icon: 'i-ph-moon-stars' },
  ],
} as const

// Educational snapshots only. PAYG purchases whole binary-GB blocks at the
// 100 MiB threshold; it is not continuous byte-by-byte wallet deduction.
// These values never enter the account cache, mutation API or checkout.
export const trafficDemoFrames = [
  { usageMiB: 0, blocks: 0, creditMiB: 0, balance: '10.00' },
  { usageMiB: 100, blocks: 1, creditMiB: 924, balance: '9.00' },
  { usageMiB: 512, blocks: 1, creditMiB: 512, balance: '9.00' },
  { usageMiB: 1536, blocks: 2, creditMiB: 512, balance: '8.00' },
  { usageMiB: 1536, blocks: 2, creditMiB: 512, balance: '8.00' },
  { usageMiB: 1536, blocks: 2, creditMiB: 0, balance: '8.00' },
] as const

// One illustrative squad at 1 TXB/hour. The activation threshold only checks
// eligibility; the half-hour charge is prorated. Extra blocks are decimal GB.
export const hourlyDemoFrames = [
  { time: '12:30', balance: '9.50', debit: '0.50', remaining: 100, icon: 'i-ph-clock' },
  { time: '13:00', balance: '8.50', debit: '1.00', remaining: 100, icon: 'i-ph-clock-clockwise' },
  { time: '13:10', balance: '8.50', debit: '0.00', remaining: 50, icon: 'i-ph-circles-three' },
  { time: '13:20', balance: '8.50', debit: '0.00', remaining: 0, icon: 'i-ph-gauge' },
  { time: '13:25', balance: '8.30', debit: '0.20', remaining: 100, icon: 'i-ph-rocket-launch' },
  { time: '13:40', balance: '8.30', debit: '0.00', remaining: 60, icon: 'i-ph-stop-circle' },
  { time: '00:00', balance: '8.30', debit: '0.00', remaining: 100, icon: 'i-ph-moon-stars' },
] as const

/** Smooth traffic counters; wallet debits still happen only at block boundaries. */
export function trafficDemoState(step: number, phase: number) {
  const current = trafficDemoFrames[step] ?? trafficDemoFrames[0]
  const previous = trafficDemoFrames[Math.max(0, step - 1)] ?? trafficDemoFrames[0]
  const usageMiB = Math.round(previous.usageMiB + (current.usageMiB - previous.usageMiB) * ease(between(phase, .12, .7)))
  const blocks = usageMiB < 100 ? 0 : Math.max(1, Math.ceil(usageMiB / 1024))
  const expired = step === 5 && phase >= .62
  return {
    usageMiB, blocks, expired,
    creditMiB: expired ? 0 : Math.max(0, blocks * 1024 - usageMiB),
    balance: (10 - blocks).toFixed(2),
    connected: step === 0 ? phase >= .3 : step === 4 ? phase >= .4 : true,
  }
}

/** UI timing remains illustrative; the values never become billing inputs. */
export function hourlyDemoState(step: number, phase: number) {
  const current = hourlyDemoFrames[step] ?? hourlyDemoFrames[0]
  const previous = hourlyDemoFrames[Math.max(0, step - 1)] ?? hourlyDemoFrames[0]
  const actionAt = step === 1 ? .62 : step === 4 ? .52 : .32
  const acted = phase >= actionAt
  const reset = step === 6 && phase >= .55
  const extra = step >= 4 && !reset
  let remaining: number = current.remaining
  if (step === 2 || step === 3) remaining = previous.remaining + (current.remaining - previous.remaining) * ease(between(phase, .08, .7))
  if (step === 4 && !acted) remaining = 0
  if (step === 5) remaining = 100 - 40 * ease(between(phase, .04, .3))
  if (step === 6 && !reset) remaining = 60
  const balance = step === 0 && !acted ? '10.00' : [1, 4].includes(step) && !acted ? previous.balance : current.balance
  const enabled = step === 0 ? acted : step < 5 || (step === 5 && phase < .32)
  const autoBuy = step > 4 || (step === 4 && phase >= .3)
  return {
    remaining, balance, enabled, autoBuy, extra, reset,
    debit: [0, 1, 4].includes(step) && acted ? current.debit : '0.00',
    time: step === 0 ? current.time : step === 1 && !acted ? '12:59' : step === 6 && !reset ? '23:59' : current.time,
    reduced: enabled && remaining <= 0,
  }
}
