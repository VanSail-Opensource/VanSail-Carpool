<script setup lang="ts">
import { computed } from 'vue'
import type { IPLookupQuote, IPLookupState } from '@/api/ipLookup'
import { useI18n } from '@/i18n'
import { formatMoney } from '@/utils/format'

const ip = defineModel<string>({ required: true })
const props = defineProps<{ state: IPLookupState; quote: IPLookupQuote | null; busy: boolean; validIp: boolean }>()
defineEmits<{ check: [] }>()
const { t } = useI18n()
const cost = computed(() => props.quote?.cachedReport ? t('ipLookup.cachedFree') : props.quote?.useQuota
  ? t('ipLookup.checkIncluded') : t('ipLookup.checkPaid', { fee: props.quote ? formatMoney(props.quote.charge) : props.state.lookupFee ? formatMoney(props.state.lookupFee) : '' }))
</script>

<template>
  <section class="lookup-input">
    <form class="lookup-form" @submit.prevent="$emit('check')">
      <UFormField name="ip" :label="$t('ipLookup.address')" class="lookup-field">
        <UInput v-model.trim="ip" class="w-full" inputmode="text" autocomplete="off" autocapitalize="off" :spellcheck="false" :placeholder="$t('ipLookup.placeholder')" />
      </UFormField>
      <UButton type="submit" class="lookup-submit" icon="i-ph-magnifying-glass" :loading="busy" :disabled="busy || !validIp || !quote || !!quote.cachedReport" :aria-label="cost" :title="cost" />
    </form>
    <div class="lookup-caption"><span><UIcon name="i-ph-coins" aria-hidden="true" />{{ cost }}</span><span v-if="state.allowance" :title="$t('ipLookup.until', { date: new Date(state.allowance.validUntil).toLocaleDateString() })"><UIcon name="i-ph-ticket" aria-hidden="true" />{{ $t('ipLookup.allowanceShort', { remaining: state.allowance.remaining, total: state.allowance.total }) }}</span></div>
  </section>
</template>

<style scoped>
.lookup-input { display: grid; gap: 0.7rem; padding: 1rem 0; border-block: 1px solid var(--line); }
.lookup-caption { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 0.25rem 1rem; color: var(--text-muted); font-size: 0.75rem; }
.lookup-caption > span { display: inline-flex; align-items: center; gap: 0.3rem; }
.lookup-form { display: grid; grid-template-columns: minmax(0, 1fr) 44px; align-items: end; gap: 0.5rem; }
.lookup-field { min-width: 0; }
.lookup-submit { width: 44px; height: 44px; }
.lookup-form :deep(input) { font-size: 16px; }
</style>
