import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import { setLocale } from '@/i18n'
import IntroSequence from './IntroSequence.vue'

describe('intro advancement recovery', () => {
  it('can retry failed advancement after the intro has completed and excludes pending clicks', async () => {
    setLocale('en')
    const wrapper = mount(IntroSequence, { props: { messages: [] }, global: { stubs: {
      LanguageControl: true, Progress: true,
    } } })
    await wrapper.get('button').trigger('click')
    expect(wrapper.emitted('complete')).toHaveLength(1)
    await wrapper.setProps({ error: 'Could not advance. Try again.' })
    expect(wrapper.text()).toContain('Could not advance. Try again.')
    expect(wrapper.get('button').text()).toBe('Try again')
    await wrapper.get('button').trigger('click')
    expect(wrapper.emitted('complete')).toHaveLength(2)
    await wrapper.setProps({ busy: true })
    expect(wrapper.get('button').attributes('disabled')).toBeDefined()
    await wrapper.get('button').trigger('click')
    expect(wrapper.emitted('complete')).toHaveLength(2)
    wrapper.unmount()
  })
})
