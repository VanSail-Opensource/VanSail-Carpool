import { mount } from '@vue/test-utils'
import { h } from 'vue'
import { describe, expect, it } from 'vitest'
import type { EarlyActivationQuote } from '@/api/types'
import { t } from '@/i18n'
import EarlyActivationDialog from './EarlyActivationDialog.vue'

const quote: EarlyActivationQuote = { currentPurchaseId: 'current', queuedPurchaseId: 'queued', eligible: true,
  currentValidUntil: '2026-12-01T00:00:00Z', newValidFrom: '2026-11-13T00:00:00Z', newValidUntil: '2026-12-13T00:00:00Z' }
const base = {
  Modal: { props: { open: Boolean }, template: '<section v-if="open"><slot name="body" /><slot name="footer" /></section>' },
  FormField: { template: '<label><slot /></label>' },
  Input: { props: ['modelValue'], emits: ['update:modelValue'], render(this: { modelValue: string; $emit: (event: string, value: string) => void }) {
    return h('input', { value: this.modelValue, onInput: (event: Event) => this.$emit('update:modelValue', (event.target as HTMLInputElement).value) })
  } },
  Button: { props: ['label', 'disabled'], render(this: { label: string; disabled: boolean }) { return h('button', { disabled: this.disabled }, this.label) } },
}
const stubs = { ...base, ...Object.fromEntries(Object.entries(base).map(([name, value]) => ['U' + name, value])) }

describe('early activation reviewed terms', () => {
  it.each(['target', 'period', 'eligibility'] as const)('retires typed consent after a changed %s', async change => {
    const wrapper = mount(EarlyActivationDialog, { props: { open: true, quote, busy: false }, global: { stubs } })
    await wrapper.get('input').setValue(t('settings.combo.confirmation'))
    expect(wrapper.findAll('button')[1]!.attributes('disabled')).toBeUndefined()
    const next = change === 'target' ? { ...quote, queuedPurchaseId: 'replacement' }
      : change === 'period' ? { ...quote, newValidUntil: '2026-12-20T00:00:00Z' } : { ...quote, eligible: false }
    await wrapper.setProps({ quote: next })
    expect(wrapper.get('input').element.value).toBe('')
    expect(wrapper.findAll('button')[1]!.attributes('disabled')).toBeDefined()
    expect(wrapper.emitted('confirm')).toBeUndefined()
    wrapper.unmount()
  })
})
