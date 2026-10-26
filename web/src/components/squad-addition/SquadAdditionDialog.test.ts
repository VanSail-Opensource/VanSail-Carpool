import { mount } from '@vue/test-utils'
import { shallowRef } from 'vue'
import { describe, expect, it, vi } from 'vitest'

import type { Purchase } from '@/api/types'

const squadAdditionMock = vi.hoisted(() => ({ useSquadAddition: vi.fn() }))
const routerMock = vi.hoisted(() => ({ push: vi.fn() }))

vi.mock('@/composables/useSquadAddition', () => squadAdditionMock)
vi.mock('@/components/catalog/useCatalogSquadPresentation', () => ({
  useCatalogSquadPresentation: () => ({ featuredIds: shallowRef([]), orderedIds: shallowRef([]) }),
}))
vi.mock('vue-router', async (importOriginal) => ({
  ...await importOriginal<typeof import('vue-router')>(),
  useRouter: () => ({ push: routerMock.push }),
}))

import SquadAdditionDialog from './SquadAdditionDialog.vue'

const active = {
  id: 'purchase-1',
  comboId: 'combo-1',
  comboName: 'Combo',
  price: { currency: 'TXB', minor: '1000', display: '10.00 TXB' },
  grossPrice: { currency: 'TXB', minor: '1000', display: '10.00 TXB' },
  couponDiscount: { currency: 'TXB', minor: '0', display: '0.00 TXB' },
  couponGrantId: null,
  validFrom: '2026-08-24T00:00:00Z',
  validUntil: '2026-09-23T00:00:00Z',
  status: 'active',
  autoRenewEnabled: false,
  trafficLimitBytes: '1073741824',
  resetStrategy: 'MONTH_ROLLING',
  squadUuids: [],
  createdAt: '2026-08-24T00:00:00Z',
  updatedAt: '2026-08-24T00:00:00Z',
} as Purchase

function mountDialog() {
  squadAdditionMock.useSquadAddition.mockReturnValue({
    selectedSquadIds: shallowRef(['squad-1']), quote: shallowRef(null), purchase: shallowRef(null),
    couponGrants: shallowRef([]), selectedCouponGrantId: shallowRef(null), eligibleCoupons: shallowRef([]),
    loading: shallowRef(false), quoting: shallowRef(false), purchasing: shallowRef(false),
    needsBalance: shallowRef(false), error: shallowRef(null), visibleSquads: shallowRef([]),
    selectedSquads: shallowRef([]), activationSquads: shallowRef([]), reset: vi.fn(),
    toggleSquad: vi.fn(), load: vi.fn(), refreshQuote: vi.fn().mockResolvedValue(true),
    confirmPurchase: vi.fn(),
  })

  return mount(SquadAdditionDialog, {
    props: { open: true, active },
    global: {
      stubs: {
        Modal: { template: '<div><slot name="body" /><slot name="footer" /></div>' },
        CatalogSquadStep: true,
        CatalogCouponStep: true,
        SquadAdditionCheckout: true,
        SquadActivationDialog: true,
      },
    },
  })
}

describe('SquadAdditionDialog', () => {
  it('loads squads when the desktop route mounts an already-open dialog', () => {
    const wrapper = mountDialog()
    const controller = squadAdditionMock.useSquadAddition.mock.results.at(-1)!.value
    expect(controller.reset).toHaveBeenCalledOnce()
    expect(controller.load).toHaveBeenCalledOnce()
    wrapper.unmount()
  })

  it('keeps Checkout inactive until squad selection is continued', async () => {
    const wrapper = mountDialog()

    expect(wrapper.findAll('[data-slot="item"]').map((item) => item.attributes('data-state'))).toEqual([
      'active',
      'inactive',
    ])

    await wrapper.get('[data-haptic="confirm"]').trigger('click')

    expect(wrapper.findAll('[data-slot="item"]').map((item) => item.attributes('data-state'))).toEqual([
      'completed',
      'active',
    ])
  })
})
