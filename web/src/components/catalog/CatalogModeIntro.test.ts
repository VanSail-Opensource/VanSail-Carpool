import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import CatalogModeIntro from './CatalogModeIntro.vue'

describe('billing mode introduction', () => {
  it('introduces both modes with concise promotional copy and a native disclosure', async () => {
    const wrapper = mount(CatalogModeIntro, { props: { mode: 'payg' } })
    expect(wrapper.get('[aria-live=polite]').text()).toBe('Connect freely. Pay as you go.')
    expect(wrapper.get('summary').text()).toBe('About PAYG')
    expect(wrapper.get('details').attributes('open')).toBeUndefined()
    expect(wrapper.get('details > p').text()).toContain('next journey')
    expect(wrapper.text()).not.toContain('10 KiB')
    await wrapper.setProps({ mode: 'combo' })
    expect(wrapper.find('details').exists()).toBe(false)
    expect(wrapper.get('[aria-live=polite]').text()).toBe('One plan. More room to explore.')
    wrapper.unmount()
  })
})
