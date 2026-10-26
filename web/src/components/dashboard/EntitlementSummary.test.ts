import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import EntitlementSummary from './EntitlementSummary.vue'
vi.mock('./PersonalPredictions.vue', () => ({ default: { template: '<section data-predictions />' } }))
describe('PAYG-only Home', () => {
 it('shows personal predictions without requiring an active Combo', () => {
  const wrapper=mount(EntitlementSummary)
  expect(wrapper.find('[data-predictions]').exists()).toBe(true)
  expect(wrapper.text()).not.toContain('No active combo')
  wrapper.unmount()
 })
})
