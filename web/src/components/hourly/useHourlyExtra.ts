import { computed, shallowRef, watch } from 'vue'
import { t } from '@/i18n'
import type { useHourly } from './useHourly'

// Settings and the exhausted-allowance shortcut review the same live policy.
// Opening a dialog never changes the preference or authorizes a purchase.
export function useHourlyExtra(hourly: ReturnType<typeof useHourly>) {
  const open = shallowRef(false)
  const reviewedVersion = shallowRef<string | null>(null)
  const consentError = shallowRef<string | null>(null)
  const price = computed(() => hourly.value.value?.policy.policy.extraGBPriceTXBMinor ?? '0')
  const enabled = computed(() => hourly.value.value?.extraPreference.extraEnabled ?? false)
  const unavailable = computed(() => price.value === '0')
  const disabled = computed(() => hourly.busy.value || !!hourly.value.value?.legacyBlocked || (!enabled.value && unavailable.value))
  const priceChanged = computed(() => {
    const data = hourly.value.value
    return !!data?.extraPreference.extraEnabled && BigInt(price.value) > BigInt(data.extraPreference.acceptedExtraPriceTXBMinor ?? '0')
  })
  const error = computed(() => consentError.value || hourly.actionError.value)

  function current(): boolean {
    const data = hourly.value.value
    return hourly.liveReady.value && !!data && !data.legacyBlocked && !unavailable.value && data.policy.version === reviewedVersion.value
  }

  watch([hourly.value, hourly.liveReady], () => {
    if (!open.value || current()) return
    open.value = false
    if (!hourly.submitting.value) consentError.value = t('hourly.conflict')
  }, { flush: 'sync' })

  async function request(value: boolean): Promise<void> {
    if (disabled.value || (value && unavailable.value)) return
    consentError.value = null
    if (!value) { await hourly.extra(false); return }
    reviewedVersion.value = hourly.value.value!.policy.version
    open.value = true
  }

  async function confirm(): Promise<void> {
    if (!open.value || !current() || hourly.busy.value) return
    if (await hourly.extra(true)) open.value = false
  }

  return { open, price, enabled, unavailable, disabled, priceChanged, error, request, confirm }
}
