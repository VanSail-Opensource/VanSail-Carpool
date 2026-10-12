import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import type { IPLookupReport } from '@/api/ipLookup'
import IPSubnetGrid from './IPSubnetGrid.vue'

type Subnet = NonNullable<IPLookupReport['subnet']>
const options = { global: { mocks: { $t: (key: string) => key }, stubs: { UButton: { template: '<div role="button"><slot /></div>' } } } }

function result(): Subnet {
  return { prefix: '8.8.8.0/24', status: 'success', reason: '', addresses: [
    { ip: '8.8.8.8', status: 'clean', proxy: 'success', abuse: 'success', ipapi: 'success', sources: [] },
    { ip: '8.8.8.10', status: 'suspected', proxy: 'success', abuse: 'success', ipapi: 'success', sources: [], ports: [{ port: 443, status: 'success', tasks: [], findings: [{ protocol: 'reality', confidence: 'suspected', evidence: 'feasible_tls_dns_mismatch' }] }] },
    { ip: '8.8.8.11', status: 'flagged', proxy: 'success', abuse: 'success', ipapi: 'success', sources: [], ports: [{ port: 443, status: 'success', tasks: [], findings: [{ protocol: 'cloudflare', confidence: 'confirmed', evidence: 'cdn_trace:SJC' }] }] },
  ] }
}

describe('subnet probe evidence', () => {
  it('shows yellow Reality and red Cloudflare with selectable evidence', async () => {
    const wrapper = mount(IPSubnetGrid, { ...options, props: { result: result(), requestedIp: '8.8.8.8' } })
    const suspected = wrapper.get('.subnet-cell.cell-suspected')
    expect(suspected.attributes('aria-label')).toContain('ipLookup.subnetStates.suspected')
    await suspected.trigger('click')
    expect(suspected.attributes('aria-pressed')).toBe('true')
    expect(wrapper.find('.subnet-probe-evidence .finding-suspected').exists()).toBe(true)
    await wrapper.get('.subnet-cell.cell-flagged').trigger('click')
    expect(wrapper.get('.selected-address').text()).toContain('8.8.8.11')
    expect(wrapper.find('.subnet-probe-evidence .finding-confirmed').exists()).toBe(true)
  })

  it('reads legacy rows without probes and clears selection when the prefix changes', async () => {
    const value = result()
    value.addresses.forEach(row => { delete row.ports })
    const wrapper = mount(IPSubnetGrid, { ...options, props: { result: value, requestedIp: '8.8.8.8' } })
    await wrapper.get('.subnet-cell.cell-suspected').trigger('click')
    expect(wrapper.find('.subnet-probe-evidence').exists()).toBe(false)
    await wrapper.setProps({ result: { ...value, prefix: '1.1.1.0/24' } })
    expect(wrapper.find('.selected-address').exists()).toBe(false)
  })
})
