import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import TxbAmountField from './TxbAmountField.vue'

describe('exact amount entry', () => {
  it('preserves a typed amount beyond binary integer precision and exposes exact minor units', async () => {
    const wrapper = mount(TxbAmountField, {
      props: { id: 'amount', label: 'Amount', modelValue: '' },
      global: { stubs: {
        UFormField: { template: '<div><slot /></div>' },
        UIcon: true,
      } },
    })
    await wrapper.get('input').setValue('90071992547409.93')
    expect(wrapper.emitted('update:modelValue')?.at(-1)).toEqual(['90071992547409.93'])
    await wrapper.setProps({ modelValue: '90071992547409.93' })
    expect(wrapper.vm.minor).toBe('9007199254740993')
    expect(wrapper.vm.valid).toBe(true)
    await wrapper.get('input').setValue('1.005')
    expect(wrapper.emitted('update:modelValue')?.at(-1)).toEqual(['1.005'])
    await wrapper.setProps({ modelValue: '1.005' })
    expect(wrapper.vm.minor).toBe('')
    expect(wrapper.vm.valid).toBe(false)
    wrapper.unmount()
  })
})
