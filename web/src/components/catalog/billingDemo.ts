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
