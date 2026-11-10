import { computed, shallowRef, watch, type Ref } from 'vue'

import type { FeaturePaymentMethod } from '@/api/features'
import { displayInputCurrency, displayInputFromTXBMinor, displayInputMinimumFromTXBMinor, displayInputToTXBMinor } from '@/utils/displayCurrency'
import { moneyFromTxbInput, txbInputFromMinor } from '@/utils/format'
import { positivePaymentMinor } from './paymentOrderHelpers'

interface PaymentConfigurationOptions {
  minimumMinor?: () => string
  maximumMinor?: () => string
}

export function usePaymentConfiguration(options: PaymentConfigurationOptions, stage: Ref<string>) {
  const amountText = shallowRef('')
  const amountMinor = shallowRef('')
  const selectedMethodId = shallowRef<string | null>(null)
  const minimumMinor = computed(() => positivePaymentMinor(options.minimumMinor?.(), 100n))
  const maximumMinor = computed(() => positivePaymentMinor(options.maximumMinor?.(), 10_000_000_000n))
  const inputCurrency = computed(displayInputCurrency)

  // The canonical TXB minor is the order amount. `amount` only carries the
  // display-currency text the member types; a currency switch re-renders that
  // text from the canonical minor instead of re-pricing the order.
  const amount = computed<string>({
    get: () => amountText.value,
    set: (value) => {
      amountText.value = value
      amountMinor.value = displayMinor(value)
    },
  })
  const amountValid = computed(() => amountMinor.value !== ''
    && BigInt(amountMinor.value) >= minimumMinor.value
    && BigInt(amountMinor.value) <= maximumMinor.value)
  const canCreate = computed(() => amountValid.value && selectedMethodId.value !== null && stage.value === 'configure')

  setAmountMinor(defaultAmountMinor())

  watch(inputCurrency, () => {
    if (amountMinor.value !== '') {
      amountText.value = displayTextForMinor(amountMinor.value)
      return
    }
    amountMinor.value = displayMinor(amountText.value)
  })

  function reset(methods: readonly FeaturePaymentMethod[]): void {
    if (!amountValid.value) setAmountMinor(defaultAmountMinor())
    const selected = methods.find((method) => method.id === selectedMethodId.value && method.available)
    selectedMethodId.value = selected?.id ?? methods.find((method) => method.available)?.id ?? null
  }

  function chooseMethod(methodId: string): void {
    if (stage.value === 'configure') selectedMethodId.value = methodId
  }

  function setAmountMinor(minor: string): void {
    if (!/^\d+$/.test(minor)) {
      amountText.value = ''
      amountMinor.value = ''
      return
    }
    amountMinor.value = minor
    amountText.value = displayTextForMinor(minor)
  }

  function displayTextForMinor(minor: string): string {
    const displayed = displayInputCurrency() === 'TXB' ? txbInputFromMinor(minor) : displayInputFromTXBMinor(minor)
    return roundsBelowMinimum(displayed, minor) ? displayInputMinimumFromTXBMinor(minor) : displayed
  }

  function roundsBelowMinimum(displayed: string, minor: string): boolean {
    const parsed = displayMinor(displayed)
    return parsed !== '' && BigInt(parsed) < minimumMinor.value && BigInt(minor) >= minimumMinor.value
  }

  function displayMinor(value: string): string {
    if (value.trim() === '') return ''
    return displayInputCurrency() === 'TXB' ? moneyFromTxbInput(value) : displayInputToTXBMinor(value)
  }

  function defaultAmountMinor(): string {
    const preferred = 2000n
    const minor = preferred < minimumMinor.value
      ? minimumMinor.value
      : preferred > maximumMinor.value ? maximumMinor.value : preferred
    return minor.toString()
  }

  return {
    amount, selectedMethodId, amountMinor, minimumMinor, maximumMinor,
    amountValid, canCreate, reset, chooseMethod, setAmountMinor,
  }
}
