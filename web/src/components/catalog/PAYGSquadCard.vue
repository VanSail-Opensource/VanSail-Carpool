<script setup lang="ts">
import type { PAYGSquad } from '@/api/payg'
import { useI18n } from '@/i18n'
import { formatBytes } from '@/utils/format'
import { formatMemberMoney } from '@/utils/displayCurrency'
import SquadPricingCard from './SquadPricingCard.vue'
import PAYGStatusDot from './PAYGStatusDot.vue'

defineProps<{ item: PAYGSquad }>()
const { t } = useI18n()
</script>

<template>
  <SquadPricingCard class="payg-squad" :squad="item.squad" :selected="false" :included="false" :featured="false" informational>
    <PAYGStatusDot :status="item.status" />
    <template #price><div class="payg-squad__price"><strong>{{ formatMemberMoney(item.squad.paygPrice ?? item.squad.price) }}</strong><small>{{ t('payg.perGB') }}</small></div></template>
    <footer class="payg-squad__footer">
      <div class="payg-squad__usage">
        <span><UIcon name="i-ph-arrows-down-up" aria-hidden="true" /><span class="sr-only">{{ t('payg.traffic') }} </span>{{ formatBytes(item.trafficBytes) }}</span>
        <span><UIcon name="i-ph-coins" aria-hidden="true" /><span class="sr-only">{{ t('payg.spent') }} </span>{{ formatMemberMoney(item.spent) }}</span>
      </div>
    </footer>
  </SquadPricingCard>
</template>

<style scoped>
.payg-squad { display: flex; flex-direction: column; align-self: stretch; gap: .4rem; padding: .6rem .6rem .6rem 1.5rem; }
.payg-squad :deep(.squad-profile-facts--member) { gap: .15rem .35rem; }
.payg-squad :deep(.squad-profile-facts--member > span) { padding-block: .12rem; }
.payg-squad__price { display: flex; flex-wrap: wrap; align-items: baseline; justify-content: flex-end; gap: .2rem; }
.payg-squad__price strong { font: 600 .95rem var(--font-mono); white-space: nowrap; }
.payg-squad__price small { color: var(--text-muted); font-size: .65rem; white-space: nowrap; }
.payg-squad__footer { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: .35rem .6rem; margin-top: auto; padding-top: .2rem; }
.payg-squad__usage { display: flex; width: 100%; justify-content: space-between; flex-wrap: wrap; gap: .65rem; font: .73rem var(--font-mono); color: var(--text-muted); }
.payg-squad__usage > span { display: inline-flex; align-items: center; gap: .25rem; }
.payg-squad__usage .iconify { font-size: .85rem; flex-shrink: 0; }
</style>
