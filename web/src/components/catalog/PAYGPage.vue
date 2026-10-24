<script setup lang="ts">
import { paygApi, type PAYGSquad } from '@/api/payg'
import InlineNotice from '@/components/common/InlineNotice.vue'
import SkeletonBlock from '@/components/common/SkeletonBlock.vue'
import { useSubscriptionData } from '@/components/subscription/useSubscriptionData'
import { useI18n } from '@/i18n'
import { formatBytes } from '@/utils/format'
import { formatMemberMoney } from '@/utils/displayCurrency'
import SquadPricingCard from './SquadPricingCard.vue'

const { t } = useI18n()
const { value: data, loading, error, refresh } = useSubscriptionData(paygApi.get, 15_000, 'payg.loadFailed')
function statusColor(status: PAYGSquad['status']) {
  return status === 'available' ? 'success' : status === 'insufficient_balance' ? 'warning' : 'neutral'
}
</script>

<template>
  <section class="payg-page">
    <div class="payg-page__intro">
      <p>{{ t('payg.automatic') }}</p>
      <div v-if="data" class="payg-page__balance"><span>{{ t('payg.balance') }}</span><strong>{{ formatMemberMoney(data.balance) }}</strong></div>
    </div>
    <InlineNotice v-if="error" tone="warning">{{ error }}</InlineNotice>
    <UButton v-if="error" color="neutral" variant="outline" :label="t('common.tryAgain')" @click="refresh()" />
    <div v-if="loading" class="payg-page__grid" aria-busy="true"><SkeletonBlock v-for="index in 3" :key="index" height="10rem" /></div>
    <template v-else-if="data">
      <p v-if="!data.squads.length" class="payg-page__empty">{{ t('payg.empty') }}</p>
      <div v-else class="payg-page__grid">
        <SquadPricingCard v-for="(item, index) in data.squads" :key="item.squad.id" class="payg-page__squad" :style="{ '--entry-delay': `${Math.min(index, 4) * 35}ms` }" :squad="item.squad" :selected="false" :included="false" :featured="false" informational :show-nodes="false">
          <template #price><strong>{{ formatMemberMoney(item.squad.paygPrice ?? item.squad.price) }}</strong><small>{{ t('payg.perGB') }}</small></template>
          <template #status><UBadge :color="statusColor(item.status)" variant="soft" :label="t(`payg.status.${item.status}`)" /></template>
          <div class="payg-page__usage"><small>{{ t('payg.window') }}</small><span>{{ formatBytes(item.trafficBytes) }}</span><strong>{{ formatMemberMoney(item.spent) }}</strong></div>
        </SquadPricingCard>
      </div>
      <footer class="payg-page__footer">
        <div class="payg-page__totals"><small>{{ t('payg.total') }}</small><strong>{{ formatBytes(data.trafficBytes) }}</strong><strong>{{ formatMemberMoney(data.spent) }}</strong></div>
        <UButton :to="{ path: '/home', query: { topUp: '1' } }" icon="i-ph-plus" color="neutral" variant="outline" :label="t('payg.addBalance')" />
      </footer>
    </template>
  </section>
</template>

<style scoped>
.payg-page { display: grid; gap: 1rem; }
.payg-page__intro { display: flex; align-items: center; justify-content: space-between; gap: 1rem; }
.payg-page__intro p, .payg-page__empty { margin: 0; color: var(--muted); font-size: 0.8rem; }
.payg-page__balance { display: grid; gap: 0.15rem; text-align: right; white-space: nowrap; font-size: 0.85rem; }
.payg-page__balance span, .payg-page__usage small, .payg-page__totals small { color: var(--muted); font-size: 0.7rem; }
.payg-page__grid { display: grid; gap: 0.7rem; grid-template-columns: repeat(auto-fit, minmax(min(100%, 19rem), 1fr)); align-items: start; }
.payg-page__squad { animation: payg-enter 180ms ease both; animation-delay: var(--entry-delay); }
.payg-page__usage { display: grid; grid-template-columns: 1fr auto auto; align-items: baseline; gap: 0.8rem; font-size: 0.82rem; font-variant-numeric: tabular-nums; }
.payg-page__footer { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 0.8rem; padding-top: 0.3rem; }
.payg-page__totals { display: flex; flex-wrap: wrap; align-items: baseline; gap: 0.8rem; font-size: 0.9rem; font-variant-numeric: tabular-nums; }
@keyframes payg-enter { from { opacity: 0; transform: translateY(6px); } to { opacity: 1; transform: translateY(0); } }
@media (max-width: 430px) { .payg-page__intro { align-items: flex-start; } .payg-page__usage { gap: 0.5rem; } }
@media (prefers-reduced-motion: reduce) { .payg-page__squad { animation: none; } }
</style>
