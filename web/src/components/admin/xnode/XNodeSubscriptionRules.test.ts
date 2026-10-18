import { defineComponent, h, reactive } from 'vue'
import { mount, flushPromises } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import UApp from '@nuxt/ui/runtime/components/App.vue'
import { setLocale, t } from '@/i18n'
import type { SubscriptionTemplateConfig } from '@/api/xnode'
const api = vi.hoisted(() => ({ previewSubscriptionSelection: vi.fn() }))
vi.mock('@/api/xnode', () => ({ xnodeApi: api }))
import XNodeSubscriptionRules from './XNodeSubscriptionRules.vue'
import { subscriptionPreset } from './subscriptionTemplatePresets'

describe('subscription selection preview lifetime', () => {
  it('aborts and ignores an old Go preview when a pattern changes', async () => {
    setLocale('en')
    let resolve!: (value: unknown) => void
    api.previewSubscriptionSelection.mockReturnValue(new Promise(done => { resolve = done }))
    const config = reactive<SubscriptionTemplateConfig>({ revision: 1, templates: [{ id: 'stash', name: 'Stash', format: 'stash', document: subscriptionPreset('stash') }], defaults: {}, assignments: { retained: { stash: 'stash' } }, formats: { stash: true }, userAgentRules: [{ pattern: 'stash', templateId: 'stash' }] })
    const wrapper = mount(defineComponent({ setup: () => () => h(UApp, null, { default: () => h(XNodeSubscriptionRules, { modelValue: config, busy: false }) }) }))
    await wrapper.findAll('button').find(button => button.text() === t('nodeTemplates.testSelection'))!.trigger('click')
    const signal = api.previewSubscriptionSelection.mock.calls[0]![1] as AbortSignal
    await wrapper.find('input').setValue('sing-box')
    resolve({ validation: { valid: true }, selection: { format: 'stash', templateId: 'stash', source: 'regex' } }); await flushPromises()
    expect(signal.aborted).toBe(true)
    expect(wrapper.find('[role="status"]').exists()).toBe(false)
    expect(config.assignments.retained).toEqual({ stash: 'stash' })
    wrapper.unmount()
  })
})
