import { mount } from '@vue/test-utils'
import { h } from 'vue'
import { describe, expect, it } from 'vitest'
import AdminSectionState from './AdminSectionState.vue'

describe('regional loading continuity', () => {
  it('retains draft input through refresh and failure, then emits retry', async () => {
    const view = mount(AdminSectionState, { props: { loading: true }, slots: { default: () => h('input', { 'aria-label': 'Draft' }) } })
    expect(view.find('input').exists()).toBe(false)
    expect(view.get('[aria-busy]').attributes('aria-busy')).toBe('true')
    expect(view.findAll('[role="status"]')).toHaveLength(1)
    await view.setProps({ loading: false })
    await view.get('input').setValue('Keep my edit')
    const input = view.get('input').element
    await view.setProps({ loading: true })
    expect(view.get('input').element).toBe(input)
    expect(input.value).toBe('Keep my edit')
    expect(view.find('.admin-loading').exists()).toBe(false)
    await view.setProps({ loading: false, error: 'Request failed' })
    expect(view.get('input').element).toBe(input)
    expect(view.text()).toContain('Request failed')
    await view.get('button').trigger('click')
    expect(view.emitted('retry')).toHaveLength(1)
    view.unmount()
  })

  it('separates initial failure from successful empty content', async () => {
    const view = mount(AdminSectionState, { props: { loading: false, error: 'Request failed' }, slots: { default: '<p data-empty>No items</p>' } })
    expect(view.find('[data-empty]').exists()).toBe(false)
    await view.setProps({ error: null })
    expect(view.get('[data-empty]').text()).toBe('No items')
    expect(view.find('.error-state').exists()).toBe(false)
    view.unmount()
  })
})
