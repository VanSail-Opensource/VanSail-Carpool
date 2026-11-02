import { mount } from '@vue/test-utils'
import { defineComponent, h } from 'vue'
import { describe, expect, it } from 'vitest'

import { setLocale } from '@/i18n'
import AdminSquadFeaturesEditor from './AdminSquadFeaturesEditor.vue'

const ButtonStub = defineComponent({
  props: { label: { type: String, default: '' }, disabled: Boolean },
  emits: ['click'],
  setup(props, { emit }) { return () => h('button', { type: 'button', disabled: props.disabled, onClick: () => emit('click') }, props.label) },
})

describe('AdminSquadFeaturesEditor', () => {
  it('adds, reorders, and removes features while preserving their details', async () => {
    setLocale('en')
    const first = { icon: 'route' as const, title: 'Transit', detail: 'Regional' }
    const second = { icon: 'shield' as const, title: 'Private' }
    const wrapper = mount(AdminSquadFeaturesEditor, {
      props: { modelValue: [first, second] },
      global: { stubs: { Button: ButtonStub, FormField: { template: '<div><slot /></div>' }, Input: true, Select: true, Icon: true, Alert: true } },
    })
    expect(wrapper.find('.feature-editor__count').exists()).toBe(false)
    await wrapper.get('[aria-label="Move feature 2 up"]').trigger('click')
    expect(wrapper.emitted('update:modelValue')?.at(-1)?.[0]).toEqual([second, first])
    await wrapper.setProps({ modelValue: [second, first] })
    await wrapper.get('[aria-label="Remove feature 1"]').trigger('click')
    expect(wrapper.emitted('update:modelValue')?.at(-1)?.[0]).toEqual([first])
    await wrapper.setProps({ modelValue: [first] })
    await wrapper.findAll('button').find(button => button.text() === 'Add feature')!.trigger('click')
    expect(wrapper.emitted('update:modelValue')?.at(-1)?.[0]).toEqual([first, { icon: 'route', title: '', detail: '' }])
  })
})
