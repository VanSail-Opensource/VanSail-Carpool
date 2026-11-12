import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import UButton from '@nuxt/ui/runtime/components/Button.vue'
import UTabs from '@nuxt/ui/runtime/components/Tabs.vue'
import { featuresApi, type OnboardingBundle, type OnboardingLocalizedContent } from '@/api/features'
import { clearResponseCache } from '@/api/cache/session'
import { t } from '@/i18n'
import AdminOnboardingPanel from './AdminOnboardingPanel.vue'
import OnboardingWelcomeEditor from './onboarding/OnboardingWelcomeEditor.vue'
import OnboardingAgreementEditor from './onboarding/OnboardingAgreementEditor.vue'

const original: OnboardingLocalizedContent = { en: [{ id: 'welcome', text: 'Original' }], 'zh-CN': [{ id: 'welcome', text: '原文' }] }
const changed: OnboardingLocalizedContent = { en: [{ id: 'welcome', text: 'Revised' }], 'zh-CN': [{ id: 'welcome', text: '修订' }] }
const bundle: OnboardingBundle = { kind: 'welcome', draft: original, published: original, draftRevision: 1, publishedRevision: 1 }
afterEach(() => { vi.restoreAllMocks(); clearResponseCache() })

describe('onboarding revision recovery', () => {
  it('saves current edits before publishing their new revision', async () => {
    vi.spyOn(featuresApi, 'getAdminOnboardingBundle').mockResolvedValue(bundle)
    const saved = { ...bundle, draft: changed, draftRevision: 2 }
    const save = vi.spyOn(featuresApi, 'saveAdminOnboardingDraft').mockResolvedValue(saved)
    const publish = vi.spyOn(featuresApi, 'publishAdminOnboarding').mockResolvedValue({ ...saved, published: changed, publishedRevision: 2 })
    const wrapper = shallowMount(AdminOnboardingPanel)
    await flushPromises()
    wrapper.getComponent(OnboardingWelcomeEditor).vm.$emit('update:content', changed)
    await wrapper.vm.$nextTick()
    wrapper.findAllComponents(UButton).find(button => button.props('label') === t('adminOnboarding.publish'))!.vm.$emit('click')
    await flushPromises()
    expect(save).toHaveBeenCalledWith('welcome', 1, changed)
    expect(publish).toHaveBeenCalledWith('welcome', 2)
    wrapper.unmount()
  })

  it('keeps a newer tab loading when an older tab response finishes', async () => {
    let finishWelcome!: (value: OnboardingBundle) => void
    let finishAgreements!: (value: OnboardingBundle) => void
    vi.spyOn(featuresApi, 'getAdminOnboardingBundle')
      .mockReturnValueOnce(new Promise(resolve => { finishWelcome = resolve }))
      .mockReturnValueOnce(new Promise(resolve => { finishAgreements = resolve }))
    const wrapper = shallowMount(AdminOnboardingPanel)
    wrapper.getComponent(UTabs).vm.$emit('update:modelValue', 'agreements')
    await wrapper.vm.$nextTick()
    finishWelcome(bundle)
    await flushPromises()
    expect(wrapper.findComponent(OnboardingWelcomeEditor).exists()).toBe(false)
    expect(wrapper.findComponent(OnboardingAgreementEditor).exists()).toBe(false)
    const content: OnboardingLocalizedContent = { en: [{ id: 'rules', icon: 'shield-check', title: 'Rules', body: 'Terms' }], 'zh-CN': [{ id: 'rules', icon: 'shield-check', title: '规则', body: '条款' }] }
    finishAgreements({ ...bundle, kind: 'agreements', draft: content, published: content })
    await flushPromises()
    expect(wrapper.getComponent(OnboardingAgreementEditor).props('content')).toEqual(content)
    wrapper.unmount()
  })
})
