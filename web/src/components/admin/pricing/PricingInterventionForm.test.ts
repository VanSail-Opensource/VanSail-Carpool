import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import USelect from '@nuxt/ui/runtime/components/Select.vue'
import { pricingAnalysisApi } from '@/api/pricingAnalysis'
import TxbAmountField from '@/components/common/TxbAmountField.vue'
import PricingInterventionForm from './PricingInterventionForm.vue'

afterEach(() => vi.restoreAllMocks())

describe('manual pricing intervention validation', () => {
  it('rejects empty, fractional minor-unit and oversized prices before submission', async () => {
    vi.spyOn(pricingAnalysisApi, 'version').mockResolvedValue({ version: 1 })
    const intervene = vi.spyOn(pricingAnalysisApi, 'intervene').mockResolvedValue({ id: 'command', status: 'ACTIVE' })
    const wrapper = shallowMount(PricingInterventionForm, { global: { renderStubDefaultSlot: true } })
    await flushPromises()
    wrapper.findAllComponents(USelect)[0]!.vm.$emit('update:modelValue', 'MANUAL_RATE')
    wrapper.getComponent({ name: 'Textarea' }).vm.$emit('update:modelValue', 'Support correction')
    await wrapper.vm.$nextTick()
    for (const value of ['', '1.005', '10000000000.01']) {
      wrapper.getComponent(TxbAmountField).vm.$emit('update:modelValue', value)
      await wrapper.vm.$nextTick(); await wrapper.get('form').trigger('submit'); await flushPromises()
    }
    expect(intervene).not.toHaveBeenCalled()
    wrapper.getComponent(TxbAmountField).vm.$emit('update:modelValue', '12.50')
    await wrapper.vm.$nextTick(); await wrapper.get('form').trigger('submit'); await flushPromises()
    expect(intervene).toHaveBeenCalledWith(expect.objectContaining({ priceMinor: '1250', command: 'MANUAL_RATE' }))
    wrapper.unmount()
  })
})
