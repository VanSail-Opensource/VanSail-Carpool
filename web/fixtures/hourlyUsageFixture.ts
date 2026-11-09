import type { components } from '@/api/generated'
type Snapshot = components['schemas']['UsageBreakdown']
const amount = (value = '0') => ({ numerator: value, denominator: '1' })
const squad = (groupId: string, name: string, uploadBytes: string, downloadBytes: string, cost = '0') => ({ groupId, name, uploadBytes, downloadBytes, costMinor: amount(cost) })
const node = (nodeId: string, name: string, bytes: string, cost: string) => ({ nodeId, name, countryCode: 'JP', uploadBytes: bytes, downloadBytes: '0', costMinor: amount(cost) })

export function hourlyUsageFixture(view: Snapshot, params: URLSearchParams): void {
  const flags = ['hourly', 'hourly-only', 'hourly-one', 'hourly-small', 'hourly-idle', 'hourly-ended', 'hourly-extra', 'hourly-omitted', 'hourly-large', 'hourly-unnamed']
  view.positiveEntries = { ...view.positiveNodes }
  if (params.has('empty') || !flags.some(flag => params.has(flag))) return
  view.nodes = [node('tokyo', 'Tokyo physical node', '8000', '80'), node('payg', 'Singapore PAYG node', '2000', '20')]
  view.hourlySquads = [squad('tokyo', 'Pacific hourly squad', '1000', '3000', '40'), squad('europe', 'Europe hourly squad', '2000', '1000', '30')]
  view.omitted = { uploadBytes: '0', downloadBytes: '0', costMinor: amount() }
  let unallocated = '0'
  if (params.has('hourly-only')) view.nodes = []
  if (params.has('hourly-one')) { view.nodes = []; view.hourlySquads = [squad('single', 'Single hourly squad', '2500', '7500', '125')] }
  if (params.has('hourly-small')) { view.nodes = [node('major', 'Primary physical node', '1000000', '1000000')]; view.hourlySquads = [squad('small', 'Small but named hourly squad', '1', '1', '1')] }
  if (params.has('hourly-idle')) { view.nodes = []; view.hourlySquads = [squad('idle', 'Idle paid hourly squad', '0', '0', '75')] }
  if (params.has('hourly-ended')) { view.nodes = []; view.hourlySquads = [squad('ended', 'Ended hourly squad', '100', '333', '32')] }
  if (params.has('hourly-extra')) unallocated = '25'
  if (params.has('hourly-large')) {
    view.nodes = [node('large', 'Large physical node', '9007199254740993', '9007199254740993')]
    view.hourlySquads = [squad('large', 'Large hourly squad', '9007199254740995', '9007199254740997', '9007199254740999')]
    unallocated = '1'
  }
  if (params.has('hourly-unnamed')) view.hourlySquads[0]!.name = ''
  if (params.has('hourly-omitted')) {
    view.nodes = Array.from({ length: 64 }, (_, index) => node(`physical-${index}`, `Physical node ${index}`, '1', '1'))
    view.hourlySquads = Array.from({ length: 64 }, (_, index) => squad(`hourly-${index}`, `Hourly squad ${index}`, '1', '0', '1'))
    view.omitted = { uploadBytes: '7', downloadBytes: '0', costMinor: amount('5') }
    view.coverage.truncated = true
    view.coverage.complete = false
  }
  const entries = [...view.nodes, ...view.hourlySquads]
  view.positiveNodes = { traffic: String(view.nodes.filter(row => BigInt(row.uploadBytes) + BigInt(row.downloadBytes) > 0n).length), cost: String(view.nodes.filter(row => BigInt(row.costMinor.numerator) > 0n).length) }
  view.positiveEntries = { traffic: String(entries.filter(row => BigInt(row.uploadBytes) + BigInt(row.downloadBytes) > 0n).length + (params.has('hourly-omitted') ? 1 : 0)), cost: String(entries.filter(row => BigInt(row.costMinor.numerator) > 0n).length + (params.has('hourly-omitted') ? 1 : 0)) }
  const cost = entries.reduce((sum, row) => sum + BigInt(row.costMinor.numerator), BigInt(unallocated) + BigInt(view.omitted.costMinor.numerator))
  view.cost = { actualMinor: cost.toString(), legacy: { status: 'none', amountMinor: null }, knownTotalMinor: amount(cost.toString()), unallocatedMinor: amount(unallocated) }
}
