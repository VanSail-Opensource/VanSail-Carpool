/* eslint-disable vue/one-component-per-file */
import { mount } from '@vue/test-utils'
import { defineComponent, h } from 'vue'
import { describe, expect, it } from 'vitest'

import type { SquadProduct } from '@/api/types'
import AdminSquadEditor from './AdminSquadEditor.vue'

const squad: SquadProduct = {
  id: 'squad', groupId: 'group', name: 'Pacific', description: '  **Existing legacy text**  ',
  price: { currency: 'TXB', minor: '100', display: '1.00 TXB' }, visible: true,
  upstreamPresent: true, stockHeldByCurrentUser: false, activationRequired: false,
  accessibleNodes: [], createdAt: '2026-11-02T00:00:00Z', updatedAt: '2026-11-02T00:00:00Z',
  profile: { type: 'international_network', portMbps: null, countryCode: 'JP', upstreamCarriers: ['Carrier'] },
  features: [{ icon: 'route', title: 'Existing feature' }],
}
const SlideoverStub = { template: '<div><slot name="body" /></div>' }
const FormStub = defineComponent({ emits: ['submit'], template: '<form @submit.prevent="$emit(\'submit\')"><slot /></form>' })
const FeaturesStub = defineComponent({ emits: ['update:modelValue'], setup(_, { emit }) { return () => h('button', { type: 'button', class: 'clear-features', onClick: () => emit('update:modelValue', []) }, 'Clear') } })

function mountEditor() {
  return mount(AdminSquadEditor, {
    props: { squad, busy: false },
    global: { stubs: {
      Slideover: SlideoverStub, Form: FormStub, FormField: { template: '<div><slot /></div>' },
      AdminSquadProfileEditor: true, AdminSquadFeaturesEditor: FeaturesStub, ProductPricingSettings: true,
      InlineNotice: true, SwitchField: true, TxbAmountField: true, Select: true, Input: true, Button: true,
    } },
  })
}

describe('AdminSquadEditor', () => {
  it('locks the reviewed fields and ignores another submit while the save is pending', async () => {
    const wrapper = mountEditor()
    await wrapper.setProps({ busy: true })
    expect(wrapper.get('fieldset.squad-editor__fields').attributes('disabled')).toBeDefined()
    await wrapper.get('form').trigger('submit')
    expect(wrapper.emitted('save')).toBeUndefined()
    wrapper.unmount()
  })

  it('hides the legacy description and preserves it while omitting untouched features', async () => {
    const wrapper = mountEditor()
    expect(wrapper.text()).not.toContain('Existing legacy text')
    expect(wrapper.find('textarea').exists()).toBe(false)
    await wrapper.get('form').trigger('submit')
    const payload = wrapper.emitted('save')?.[0]?.[0] as Record<string, unknown>
    expect(payload.description).toBe(squad.description)
    expect(payload).not.toHaveProperty('features')
  })

  it('sends an explicit empty list when the administrator clears features', async () => {
    const wrapper = mountEditor()
    await wrapper.get('.clear-features').trigger('click')
    await wrapper.get('form').trigger('submit')
    expect(wrapper.emitted('save')?.[0]?.[0]).toMatchObject({ description: squad.description, features: [] })
  })
})
