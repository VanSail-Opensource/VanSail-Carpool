import { computed, shallowRef, watch, type ShallowRef } from 'vue'
import type { Catalog } from '@/api/types'
import { usePurchaseValidity } from './usePurchaseValidity'

/** Catalog metadata is display-only; quote and checkout recheck admission. */
export function useCatalogAdmission(catalog: ShallowRef<Catalog | null>, refresh: () => Promise<void>) {
  const rejected = shallowRef(false)
  const deadline = computed(() => Date.parse(catalog.value?.autoRenewalBlockedUntil ?? ''))
  const validity = usePurchaseValidity(() => catalog.value?.autoRenewalBlockedUntil
    ? [{ status: 'active', validFrom: '1970-01-01T00:00:00Z', validUntil: catalog.value.autoRenewalBlockedUntil }]
    : [], refresh)
  const blocked = computed(() => rejected.value || deadline.value > validity.now.value)
  watch(catalog, () => {
    rejected.value = false
  })
  return { blocked, reject: () => { rejected.value = true } }
}
