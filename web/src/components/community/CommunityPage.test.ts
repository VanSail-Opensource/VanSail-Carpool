import { flushPromises, mount } from '@vue/test-utils'
import { createPinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import type { CommunityMembership } from '@/api/types'
import CommunityPage from './CommunityPage.vue'

const api = vi.hoisted(() => ({ checkCommunityMembership: vi.fn(), createCommunityInvite: vi.fn() }))
vi.mock('@/api/client', () => ({ api }))
vi.mock('@/utils/telegram', () => ({ getTelegramWebApp: () => undefined, notifyHaptic: vi.fn(), openExternalLink: vi.fn() }))

const access: CommunityMembership = {
  activeCombo: true, groupJoined: true, channelJoined: false,
  user: {
    id: 'user-1', telegramId: '42', firstName: 'Mira', lastName: '', telegramUsername: '', displayName: 'Mira', username: 'mira', role: 'user',
    onboardingState: 'complete', groupJoined: true, channelJoined: false, policyAcceptedAt: '2026-08-31T00:00:00Z',
    agreementRevision: 1, recoveryReason: '', createdAt: '2026-08-31T00:00:00Z', updatedAt: '2026-08-31T00:00:00Z',
  },
}

function mountPage() {
  return mount(CommunityPage, {
    global: {
      plugins: [createPinia()],
      stubs: {
        InlineNotice: { template: '<div class="notice"><slot /></div>' }, SkeletonBlock: { template: '<div class="skeleton" />' },
        CommunityMembershipRows: { props: ['activeCombo', 'groupJoined', 'channelJoined'], template: '<output data-testid="rows" :data-group="String(groupJoined)" :data-active="String(activeCombo)" />' },
      },
      mocks: { $t: (key: string) => key },
    },
  })
}

describe('CommunityPage', () => {
  beforeEach(() => vi.clearAllMocks())

  it('shows loading before rendering canonical membership rows', async () => {
    api.checkCommunityMembership.mockResolvedValue(access)
    const wrapper = mountPage()
    expect(wrapper.text()).toContain('community.loadingTitle')
    await flushPromises()

    const rows = wrapper.get('[data-testid="rows"]')
    expect(rows.attributes('data-group')).toBe('true')
    expect(rows.attributes('data-active')).toBe('true')
    wrapper.unmount()
  })

  it('keeps a localized error and refresh fallback when loading fails', async () => {
    api.checkCommunityMembership.mockRejectedValue(new Error('offline'))
    const wrapper = mountPage()
    await flushPromises()

    expect(wrapper.text()).toContain('refresh community access')
    expect(wrapper.get('.community-page__refresh').text()).toBe('common.refresh')
    wrapper.unmount()
  })
})
