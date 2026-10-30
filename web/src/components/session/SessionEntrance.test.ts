import { mount } from '@vue/test-utils'
import { h } from 'vue'
import { afterEach, describe, expect, it, vi } from 'vitest'
import SessionEntrance from './SessionEntrance.vue'

afterEach(() => vi.useRealTimers())

describe('session readiness', () => {
  it('waits only for the permitted route, while errors bypass that wait', async () => {
    const view = mount(SessionEntrance, { props: { status: 'ready', authenticated: true, routeReady: false }, slots: { default: () => h('button', 'Destination') } })
    expect(view.find('button').exists()).toBe(false)
    await view.setProps({ routeReady: true })
    expect(view.find('button').exists()).toBe(true)
    await view.setProps({ status: 'error', routeReady: false })
    expect(view.find('button').exists()).toBe(true)
    view.unmount()
  })

  it.each(['ready', 'error'] as const)('releases %s content immediately, before any intro duration', async status => {
    vi.useFakeTimers()
    const view = mount(SessionEntrance, { props: { status: 'loading' }, slots: { default: () => h('button', 'Destination') } })
    expect(view.find('button').exists()).toBe(false)
    await view.setProps({ status })
    expect(view.get('button').text()).toBe('Destination')
    expect(view.find('.session-loading').exists()).toBe(false)
    view.unmount()
  })

  it('preserves the mounted destination and draft during an authenticated refresh', async () => {
    const view = mount(SessionEntrance, { props: { status: 'ready', authenticated: true }, slots: { default: () => h('input', { 'aria-label': 'Draft' }) } })
    await view.get('input').setValue('Unsaved draft')
    const input = view.get('input').element
    await view.setProps({ status: 'loading' })
    expect(view.get('input').element).toBe(input)
    expect(input.value).toBe('Unsaved draft')
    expect(view.find('.session-loading').exists()).toBe(false)
    view.unmount()
  })

  it('replaces a single pending status after five seconds and disposes its timer', async () => {
    vi.useFakeTimers()
    const view = mount(SessionEntrance, { props: { status: 'loading' } })
    const initial = view.get('[role="status"]').text()
    await vi.advanceTimersByTimeAsync(4999)
    expect(view.get('[role="status"]').text()).toBe(initial)
    await vi.advanceTimersByTimeAsync(1)
    expect(view.findAll('[role="status"]')).toHaveLength(1)
    expect(view.get('[role="status"]').text()).not.toBe(initial)
    view.unmount()
    expect(vi.getTimerCount()).toBe(0)
  })
})
