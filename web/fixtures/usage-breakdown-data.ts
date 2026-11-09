import type { components } from '@/api/generated'
import { hourlyUsageFixture } from './hourlyUsageFixture'

type Snapshot = components['schemas']['UsageBreakdown']
const amount = (numerator = '0', denominator = '1') => ({ numerator, denominator })
const node = (nodeId: string, name: string, uploadBytes: string, downloadBytes: string, costMinor = amount()) => ({ nodeId, name, countryCode: 'JP', uploadBytes, downloadBytes, costMinor })

export function usageBreakdownFixture(params = new URLSearchParams()): Snapshot {
  const now = Date.now()
  const legacy: Snapshot['cost']['legacy'] = { status: 'available', amountMinor: amount('100', '3') }
  const view: Snapshot = {
    from: new Date(now - 72 * 3600000).toISOString(), to: new Date(now - (params.has('stale') ? 300000 : 0)).toISOString(),
    coverage: { since: new Date(now - 96 * 3600000).toISOString(), complete: true, gapCount: '0', truncated: false },
    nodes: [node('tokyo', 'Tokyo primary', '4000000000', '2000000000', amount('200')),
      node('osaka', 'Osaka shared', '1000000000', '3000000000', amount('220', '3')),
      node('historical', 'Historical node with a deliberately long name for narrow layouts', '500000000', '500000000')],
    hourlySquads: [],
    omitted: { uploadBytes: '0', downloadBytes: '0', costMinor: amount() },
    positiveNodes: { traffic: '3', cost: '2' },
    positiveEntries: { traffic: '3', cost: '2' },
    cost: { actualMinor: '300', legacy, knownTotalMinor: amount('1000', '3'), unallocatedMinor: amount('60') },
  }
  function clearEstimate(): void { view.cost.legacy = { status: 'none', amountMinor: null } }
  if (params.has('large')) {
    view.nodes = [node('tokyo', 'Tokyo exact', '9007199254740993', '0', amount('9007199254740993', '2')),
      node('osaka', 'Osaka fractional', '18014398509481986', '0', amount('27021597764222981', '6'))]
    view.positiveNodes = { traffic: '2', cost: '2' }
    view.cost = { actualMinor: '9007199254740993', legacy: { status: 'available', amountMinor: amount('1', '3') }, knownTotalMinor: amount('27021597764222980', '3'), unallocatedMinor: amount() }
  }
  if (params.has('unknown-term')) {
    view.cost = { actualMinor: '300', legacy: { status: 'original_term_unknown', amountMinor: null }, knownTotalMinor: amount('300'), unallocatedMinor: amount('60') }
    view.nodes[0]!.costMinor = amount('180'); view.nodes[1]!.costMinor = amount('60')
  }
  if (params.has('one') || params.has('one-traffic') || params.has('one-cost')) {
    clearEstimate(); view.cost.unallocatedMinor = amount()
    if (params.has('one') || params.has('one-traffic')) {
      for (const item of view.nodes.slice(1)) { item.uploadBytes = '0'; item.downloadBytes = '0' }
      view.positiveNodes.traffic = '1'
    }
    view.nodes[0]!.costMinor = amount('100')
    view.nodes[1]!.costMinor = params.has('one-traffic') ? amount('50') : amount()
    view.cost.actualMinor = params.has('one-traffic') ? '150' : '100'
    view.cost.knownTotalMinor = amount(view.cost.actualMinor)
    view.positiveNodes.cost = params.has('one-traffic') ? '2' : '1'
  }
  if (params.has('omitted') || params.has('truncated')) {
    clearEstimate()
    view.nodes = [node('tokyo', 'Tokyo retained', '4000000000', '2000000000', amount('100'))]
    view.omitted = { uploadBytes: '1000000000', downloadBytes: '3000000000', costMinor: amount('50') }
    view.positiveNodes = { traffic: '129', cost: '129' }
    view.cost.actualMinor = '175'; view.cost.knownTotalMinor = amount('175'); view.cost.unallocatedMinor = amount('25')
    view.coverage.truncated = true
    view.coverage.complete = false
  }
  if (params.has('grouping')) {
    clearEstimate()
    view.nodes = [node('major', 'Tokyo primary', '9800', '0', amount('5000')), node('threshold', 'Osaka threshold', '100', '0', amount('4900')),
      node('small', 'Frankfurt small', '99', '0', amount('99')), node('tiny', 'London tiny', '1', '0', amount('1'))]
    view.positiveNodes = { traffic: '4', cost: '4' }
    view.cost.actualMinor = '10000'; view.cost.knownTotalMinor = amount('10000'); view.cost.unallocatedMinor = amount()
  }
  if (params.has('tiny')) {
    clearEstimate()
    view.nodes[0]!.costMinor = amount('10000000000000000000000')
    for (const item of view.nodes.slice(1)) item.costMinor = amount('1')
    view.positiveNodes.cost = '3'
    view.cost.actualMinor = '10000000000000000000003'; view.cost.knownTotalMinor = amount(view.cost.actualMinor); view.cost.unallocatedMinor = amount('1')
  }
  if (params.has('empty') || params.has('unallocated')) {
    clearEstimate()
    view.nodes = []
    view.positiveNodes = { traffic: '0', cost: '0' }
    view.cost.actualMinor = '0'
    view.cost.knownTotalMinor = amount(params.has('empty') ? '0' : '300')
    view.cost.unallocatedMinor = amount(params.has('empty') ? '0' : '300')
    if (params.has('unallocated')) view.cost.legacy = { status: 'available', amountMinor: amount('300') }
  }
  if (params.has('incomplete') || params.has('gaps')) {
    view.coverage.complete = false
    view.coverage.since = params.has('no-since') ? null : new Date(now - 6 * 3600000).toISOString()
    view.coverage.gapCount = params.has('gaps') ? '2' : '0'
  }
  hourlyUsageFixture(view, params)
  return view
}
