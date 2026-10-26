<script setup lang="ts">
import { toRef } from 'vue'
import { useI18n } from '@/i18n'
import { formatMoney, formatDate, txbInputFromMinor } from '@/utils/format'
import InlineNotice from '@/components/common/InlineNotice.vue'
import PersonalPredictions from '@/components/dashboard/PersonalPredictions.vue'
import PricingDecisionFacts from './PricingDecisionFacts.vue'
import { useUserPrices } from './useUserPrices'
const props = defineProps<{ userId: string }>()
const { t } = useI18n()
const { items, cursor, loading, error, load } = useUserPrices(toRef(props, 'userId'))
</script>
<template>
  <section class="user-prices">
    <header><h2>{{ t('userPrices.title') }}</h2><UButton variant="outline" color="neutral" :loading="loading" :label="t('pricing.refresh')" @click="load()" /></header>
    <p>{{ t('userPrices.hint') }}</p>
    <PersonalPredictions :key="userId" :user-id="userId" />
    <InlineNotice v-if="error" tone="warning">{{ error }}</InlineNotice>
    <USkeleton v-if="loading && !items.length" class="h-64 w-full" />
    <p v-if="!loading && !error && !items.length">{{ t('pricing.emptyProducts') }}</p>
    <article v-for="item in items" :key="`${item.productType}:${item.productId}`" class="user-prices__product">
      <header><h3>{{ item.name }}</h3><span>{{ t(`pricing.types.${item.productType}`) }} · {{ t(item.billingMode === 'payg' ? 'pricing.catalogPayg' : 'pricing.catalogCombo') }}</span></header>
      <dl class="user-prices__amounts">
        <div v-for="field in (['fixedPrice', 'publicPrice', 'personalPrice', 'campaignPrice', 'netPrice'] as const)" :key="field"><dt>{{ t(`userPrices.${field}`) }}</dt><dd>{{ formatMoney(item[field]) }}<small v-if="item.billingMode === 'payg'">{{ t('payg.perGB') }}</small></dd></div>
      </dl>
      <PricingDecisionFacts :decision="item.decision" />
      <section><h4>{{ t('userPrices.coupons') }}</h4><p v-if="!item.coupons.length">{{ t('userPrices.noCoupons') }}</p><ul v-else class="user-prices__coupons"><li v-for="grant in item.coupons" :key="grant.id"><strong>{{ grant.coupon.name }}</strong><code>{{ grant.id }}</code><span>{{ t('userPrices.couponTerms', { kind: grant.coupon.kind, mode: grant.coupon.discountMode ?? t('common.notAvailable'), value: grant.coupon.discountMode === 'percent' ? `${Number(grant.coupon.valueMinorOrBps) / 100}%` : `${txbInputFromMinor(grant.coupon.valueMinorOrBps)} TXB`, cap: grant.coupon.percentCapMinor ? `${txbInputFromMinor(grant.coupon.percentCapMinor)} TXB` : t('common.notAvailable'), expires: grant.coupon.expiresAt ? formatDate(grant.coupon.expiresAt) : t('common.notAvailable') }) }}</span><span>{{ t('userPrices.scope', { combos: grant.coupon.eligibleComboIds.join(', ') || t('userPrices.all'), squads: grant.coupon.eligibleSquadIds.join(', ') || t('userPrices.all') }) }}</span></li></ul></section>
      <section v-if="item.renewalDecision"><h4>{{ t('pricing.renewal') }}</h4><p>{{ formatMoney(item.renewalDecision.netPrice) }}</p><PricingDecisionFacts :decision="item.renewalDecision" /></section>
    </article>
    <p>{{ t('userPrices.loaded', { count: items.length }) }}</p><UButton v-if="cursor" variant="outline" color="neutral" :loading="loading" :label="t('userPrices.loadMore')" @click="load(true)" />
  </section>
</template>
<style scoped>
.user-prices { display: grid; min-width: 0; gap: 1.5rem; }.user-prices header { display: flex; align-items: center; justify-content: space-between; flex-wrap: wrap; gap: .75rem; }.user-prices h2 { font-size: 1.1rem; }.user-prices h3 { font-size: 1rem; }.user-prices h2, .user-prices h3, .user-prices h4, .user-prices p { margin: 0; }.user-prices h4 { font-size: .85rem; margin-bottom: .75rem; }.user-prices p, .user-prices header span { font-size: .8rem; color: var(--text-muted); }.user-prices__product { display: grid; gap: 1.25rem; border-top: 1px solid var(--line); padding-top: 1.5rem; }.user-prices__amounts { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, 10rem), 1fr)); gap: 1rem; margin: 0; }.user-prices__amounts dt { color: var(--text-muted); font-size: .75rem; }.user-prices__amounts dd { margin: .25rem 0 0; font-variant-numeric: tabular-nums; }.user-prices__amounts small { margin-inline-start: .25rem; }.user-prices__coupons { display: grid; gap: 1rem; padding: 0; list-style: none; margin: 0; }.user-prices__coupons li { display: grid; gap: .25rem; font-size: .8rem; overflow-wrap: anywhere; }
</style>
