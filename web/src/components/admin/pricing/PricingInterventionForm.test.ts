import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import USelect from '@nuxt/ui/runtime/components/Select.vue'
import { pricingAnalysisApi } from '@/api/pricingAnalysis'
import TxbAmountField from '@/components/common/TxbAmountField.vue'
import PricingInterventionForm from './PricingInterventionForm.vue'
import { t } from '@/i18n'

afterEach(() => vi.restoreAllMocks())

describe('manual pricing intervention validation', () => {
  it('retains confirmed success when only the next version read fails', async () => {
    vi.spyOn(pricingAnalysisApi, 'version').mockResolvedValueOnce({ version: 1 }).mockRejectedValueOnce(new Error('read unavailable'))
    vi.spyOn(pricingAnalysisApi, 'intervene').mockResolvedValue({ id: 'command', status: 'ACTIVE' })
    const wrapper = shallowMount(PricingInterventionForm, { global: { renderStubDefaultSlot: true } })
    await flushPromises()
    wrapper.getComponent({ name: 'Textarea' }).vm.$emit('update:modelValue', 'Reviewed freeze')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(wrapper.emitted('changed')).toHaveLength(1)
    expect(wrapper.text()).toContain(t('pricing.states.ACTIVE'))
    expect(wrapper.text()).toContain(t('pricing.loadError'))
    expect(wrapper.text()).not.toContain(t('pricing.saveError'))
    expect(wrapper.getComponent({ name: 'Textarea' }).props('modelValue')).toBe('')
    wrapper.unmount()
  })

  it('does not refresh or emit into a departed workspace after a pending command', async () => {
    const version = vi.spyOn(pricingAnalysisApi, 'version').mockResolvedValue({ version: 1 })
    let finish!: (value: { id: string; status: string }) => void
    vi.spyOn(pricingAnalysisApi, 'intervene').mockReturnValue(new Promise(resolve => { finish = resolve }))
    const wrapper = shallowMount(PricingInterventionForm, { global: { renderStubDefaultSlot: true } })
    await flushPromises()
    wrapper.getComponent({ name: 'Textarea' }).vm.$emit('update:modelValue', 'Reviewed freeze')
    await wrapper.get('form').trigger('submit')
    wrapper.unmount()
    finish({ id: 'command', status: 'ACTIVE' })
    await flushPromises()
    expect(version).toHaveBeenCalledOnce()
    expect(wrapper.emitted('changed')).toBeUndefined()
  })

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
