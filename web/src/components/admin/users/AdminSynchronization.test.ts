import { defineComponent } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import type { AdminUserDetail } from '@/api/adminOperations'
import type { AdminUserSummary } from '@/api/types'
import AdminUsersPanel from '../AdminUsersPanel.vue'
import AdminUserOverview from './AdminUserOverview.vue'

const state = vi.hoisted(() => ({ items: [] as AdminUserSummary[] }))
vi.mock('@/composables/useAdminSection', () => ({
  useAdminSection: () => ({
    items: state.items, nextCursor: null, loading: false, error: null, load: vi.fn(), loadMore: vi.fn(),
  }),
}))
vi.mock('@/api/cache/catalogOptions', () => ({ restoreCatalogOptions: vi.fn() }))
vi.mock('@/api/adminOperations', () => ({
  adminOperationsApi: { getCatalogOptions: vi.fn(async () => ({ combos: [], squads: [] })) },
}))
vi.mock('vue-router', () => ({ useRouter: () => ({ push: vi.fn() }) }))
vi.mock('@/i18n', async (importOriginal) => ({
  ...await importOriginal<typeof import('@/i18n')>(),
  useI18n: () => ({ t: (key: string) => key }),
}))

const StatusBadgeStub = defineComponent({
  props: ['tone', 'label'],
  template: '<span data-sync :data-tone="tone">{{ label }}</span>',
})
const TableStub = defineComponent({
  props: { data: { type: Array, default: () => [] } },
  template: '<div><div v-for="(item, index) in data" :key="index"><slot name="status-cell" :row="{ original: item }" /></div></div>',
})
const stubs = {
  StatusBadge: StatusBadgeStub, UTable: TableStub,
  AdminSectionState: { template: '<div><slot /></div>' },
  AdminUserSearchFilters: true, AdminBulkExtensionDialog: true, InlineNotice: true,
  UInput: true, UButton: true, UAvatar: true, UIcon: true, UAlert: true,
}
const cases = [
  { status: 'synchronized', tone: 'success' },
  { status: 'pending', tone: 'warning' },
  { status: 'failed', tone: 'danger' },
  { status: 'not_provisioned', tone: 'warning' },
] as const

function summary(status: AdminUserSummary['synchronization']['status']): AdminUserSummary {
  const createdAt = '2026-11-12T00:00:00Z'
  return {
    user: {
      id: 'member-1', telegramId: '123', displayName: 'Member', firstName: 'Member',
      lastName: '', telegramUsername: '', username: null, role: 'user',
      onboardingState: 'complete', groupJoined: false, channelJoined: false,
      policyAcceptedAt: createdAt, agreementRevision: 1, recoveryReason: '',
      createdAt, updatedAt: createdAt,
    },
    balance: { currency: 'TXB', minor: '10000', display: '100.00 TXB' },
    createdAt, synchronization: { status },
  }
}

describe('administrator native synchronization display', () => {
  beforeEach(() => { state.items = [] })

  it.each(cases)('keeps $status visible in a funded member list', async ({ status, tone }) => {
    state.items = [summary(status)]
    const wrapper = mount(AdminUsersPanel, { global: { stubs } })
    await flushPromises()

    expect(wrapper.get('[data-sync]').text()).toBe('adminUserProfile.sync.' + status)
    expect(wrapper.get('[data-sync]').attributes('data-tone')).toBe(tone)
    wrapper.unmount()
  })

  it.each(cases)('keeps $status visible in a funded member profile', ({ status, tone }) => {
    const { user, balance } = summary(status)
    const detail: AdminUserDetail = {
      user: { ...user }, balance: { ...balance }, synchronization: { status },
      activeCombo: null, entitlements: [], embyAccounts: [], payments: [],
      refunds: [], operations: [], ipBlocks: [], couponWallet: [],
      affiliateHistory: { items: [], page: 1, pageSize: 5, total: 0, totalPages: 0 },
      temporaryBan: null, pmConversation: null,
    }
    const wrapper = mount(AdminUserOverview, { props: { detail }, global: { stubs } })

    expect(wrapper.get('[data-sync]').text()).toBe('adminUserProfile.sync.' + status)
    expect(wrapper.get('[data-sync]').attributes('data-tone')).toBe(tone)
    wrapper.unmount()
  })
})
