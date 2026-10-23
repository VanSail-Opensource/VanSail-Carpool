import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import type { ConnectivityAttempt } from '@/api/connectivity'
import ConnectivityAttemptSummary from './ConnectivityAttemptSummary.vue'

describe('connectivity result privacy', () => {
  it('keeps timestamps and status while hiding timing and numeric HTTP results', () => {
    const attempt = { id: 'attempt', runId: 'run', hostUuid: 'host', proxyUserId: 42, trigger: 'manual', startedAt: '2026-10-09T12:00:00Z', finishedAt: '2026-10-09T12:00:01Z', status: 'connected', latencyMs: 1234, httpStatus: 204, errorCode: '' } as ConnectivityAttempt
    const wrapper = mount(ConnectivityAttemptSummary, { props: { attempt } })
    expect(wrapper.text()).not.toContain('1234')
    expect(wrapper.text()).not.toContain('HTTP 204')
    expect(wrapper.find('small').exists()).toBe(true)
    wrapper.unmount()
  })
})
