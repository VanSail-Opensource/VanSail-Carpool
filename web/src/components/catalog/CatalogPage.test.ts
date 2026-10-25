import { mount } from '@vue/test-utils'
import { createPinia } from 'pinia'
import { createMemoryHistory, createRouter } from 'vue-router'
import { nextTick, shallowRef } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const catalogMock = vi.hoisted(() => ({ useCatalog: vi.fn() }))

vi.mock('@/composables/useCatalog', () => catalogMock)
vi.mock('./useCatalogSquadPresentation', () => ({
  useCatalogSquadPresentation: () => ({ featuredIds: [], orderedIds: [] }),
}))
vi.mock('@/stores/session', () => ({
  useSessionStore: () => ({ user: { id: 'user-1' } }),
}))

import CatalogPage from './CatalogPage.vue'

const combo = {
  id: 'combo-1',
  name: 'Combo',
  description: '',
  price: { currency: 'TXB' as const, minor: '1000', display: '10.00 TXB' },
  validityDays: 30,
  trafficLimitBytes: '1',
  resetStrategy: 'MONTH_ROLLING' as const,
  includedSquads: [],
  active: true,
  rolloverMinRemainingBps: 0,
  createdAt: '2026-08-08T00:00:00Z',
  updatedAt: '2026-08-08T00:00:00Z',
}

async function mountPage(purchase: object | null, step = 4, confirmPurchase = vi.fn(), accessibleNodes: object[] | null = null) {
  const refreshQuote = vi.fn().mockResolvedValue(true)
  const autoRenewalBlocked = shallowRef(false)
  catalogMock.useCatalog.mockReturnValue({
    catalog: shallowRef({ combos: [combo], addons: [], nodes: [] }),
    balance: shallowRef({ currency: 'TXB', minor: '1000', display: '10.00 TXB' }),
    loading: shallowRef(false),
    purchasing: shallowRef(false),
    quoting: shallowRef(false),
    error: shallowRef(null),
    purchase: shallowRef(purchase),
    quote: shallowRef(accessibleNodes ? { accessibleNodes } : null),
    quoteUsable: shallowRef(Boolean(accessibleNodes)),
    autoRenewalBlocked,
    selectedComboId: shallowRef(combo.id),
    selectedSquadIds: shallowRef<string[]>([]),
    selectedCouponGrantId: shallowRef<string | null>(null),
    couponDiscarding: shallowRef(false),
    needsBalance: shallowRef(false),
    visibleCombos: shallowRef([combo]),
    visibleSquads: shallowRef([]),
    selectedCombo: shallowRef(combo),
    selectedSquads: shallowRef([]),
    activationSquads: shallowRef([]),
    includedSquadIds: shallowRef<string[]>([]),
    couponGrants: shallowRef([]),
    eligibleCoupons: shallowRef([]),
    load: vi.fn().mockResolvedValue(undefined),
    selectCombo: vi.fn(),
    toggleSquad: vi.fn(),
    refreshQuote,
    discardCoupon: vi.fn(),
    confirmPurchase,
  })

  sessionStorage.setItem('txc-catalog-step:v2:user-1', String(step))
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/catalog', component: { template: '<div />' } },
      { path: '/home', component: { template: '<div />' } },
    ],
  })
  await router.push('/catalog')
  await router.isReady()
  const wrapper = mount(CatalogPage, {
    global: {
      plugins: [createPinia(), router],
      stubs: {
        CatalogConfirmation: { template: '<div data-test="catalog-confirmation" />' },
        CatalogComboPricingTable: { template: '<div data-test="combo-step" />' },
        CatalogCheckout: { template: '<div role="button" tabindex="0" data-test="confirm-purchase" @click="$emit(\'confirm\')" />' },
        CatalogCouponStep: { template: '<div data-test="catalog-coupon-step" />' },
        CatalogFlowControls: { props: ['nextDisabled'], emits: ['next'], template: '<div role="button" tabindex="0" data-test="catalog-flow-controls" :data-disabled="String(nextDisabled)" @click="$emit(\'next\')" />' },
        CatalogFlowProgress: { template: '<div data-test="catalog-flow-progress" />' },
        CatalogSquadStep: { template: '<div data-test="squad-step" />' },
      },
    },
  })
  await nextTick()
  return { autoRenewalBlocked, refreshQuote, router, wrapper }
}

describe('CatalogPage quote restoration', () => {
  beforeEach(() => {
    sessionStorage.clear()
    catalogMock.useCatalog.mockReset()
    window.Telegram = undefined
  })
  afterEach(() => {
    document.querySelector('.app-frame__content')?.remove()
    vi.unstubAllGlobals()
  })

  it('scrolls to the top after continuing from core combos', async () => {
    const scrollTo = vi.fn()
    const contentScrollTo = vi.fn()
    const appContent = document.createElement('div')
    appContent.className = 'app-frame__content'
    Object.defineProperty(appContent, 'scrollTo', { configurable: true, value: contentScrollTo })
    document.body.append(appContent)
    vi.stubGlobal('scrollTo', scrollTo)
    const { wrapper } = await mountPage(null, 2, vi.fn(), [{}])

    await wrapper.get('[data-test="catalog-flow-controls"]').trigger('click')

    expect(scrollTo).toHaveBeenCalledWith({ top: 0, left: 0, behavior: 'auto' })
    expect(contentScrollTo).toHaveBeenCalledWith({ top: 0, left: 0, behavior: 'auto' })
    expect(wrapper.find('[data-test="catalog-coupon-step"]').exists()).toBe(true)
    wrapper.unmount()
  })

  it('does not request another quote after purchase confirmation', async () => {
    const { refreshQuote, wrapper } = await mountPage({ id: 'purchase-1' })

    await nextTick()

    expect(refreshQuote).not.toHaveBeenCalled()
    expect(wrapper.find('[data-test="catalog-confirmation"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="catalog-flow-progress"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="catalog-flow-controls"]').exists()).toBe(false)
    wrapper.unmount()
  })

  it('restores the quote after returning to the coupon step', async () => {
    const { refreshQuote, wrapper } = await mountPage(null, 3)

    await nextTick()

    expect(refreshQuote).toHaveBeenCalled()
    wrapper.unmount()
  })

  it('clears the persisted step after a successful purchase', async () => {
    const confirmPurchase = vi.fn().mockResolvedValue(true)
    const { wrapper } = await mountPage(null, 4, confirmPurchase)

    await wrapper.get('[data-test="confirm-purchase"]').trigger('click')
    await nextTick()

    expect(confirmPurchase).toHaveBeenCalledOnce()
    expect(sessionStorage.getItem('txc-catalog-step:v2:user-1')).toBeNull()
    wrapper.unmount()
  })

  it('returns from purchase review through Telegram native Back', async () => {
    const onClick = vi.fn()
    window.Telegram = { WebApp: {
      version: '9.0', initData: 'signed', initDataUnsafe: {}, colorScheme: 'dark',
      ready: vi.fn(), expand: vi.fn(), close: vi.fn(), openLink: vi.fn(), openTelegramLink: vi.fn(), openInvoice: vi.fn(),
      BackButton: { isVisible: false, show: vi.fn(), hide: vi.fn(), onClick, offClick: vi.fn() },
    } }
    const { wrapper } = await mountPage(null, 4)

    expect(onClick).toHaveBeenCalledOnce()
    onClick.mock.calls[0]?.[0]?.()
    await nextTick()

    expect(wrapper.find('[data-test="catalog-coupon-step"]').exists()).toBe(true)
    wrapper.unmount()
  })

  it('disables Coupon-step continuation without a current usable quote', async () => {
    const { wrapper } = await mountPage(null, 3)

    expect(wrapper.get('[data-test="catalog-flow-controls"]').attributes('data-disabled')).toBe('true')
    wrapper.unmount()
  })

  it('refreshes the quote and blocks leaving core combos without accessible nodes', async () => {
    const { refreshQuote, wrapper } = await mountPage(null, 2)

    await wrapper.get('[data-test="catalog-flow-controls"]').trigger('click')

    expect(refreshQuote).toHaveBeenCalledOnce()
    expect(wrapper.find('[data-test="combo-step"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="catalog-coupon-step"]').exists()).toBe(false)
    wrapper.unmount()
  })

  it('keeps blocked members in Combo mode without redirecting', async () => {
    const { autoRenewalBlocked, router, wrapper } = await mountPage(null, 1)
    autoRenewalBlocked.value = true
    await nextTick()
    expect(router.currentRoute.value.path).toBe('/catalog')
    expect(wrapper.findComponent({ name: 'CatalogBlocked' }).exists()).toBe(true)
    expect(wrapper.find('[data-test="catalog-flow-controls"]').exists()).toBe(false)
    wrapper.unmount()
  })
})
