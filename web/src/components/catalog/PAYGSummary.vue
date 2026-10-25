<script setup lang="ts">
import type { PAYGView } from '@/api/payg'
import { useI18n } from '@/i18n'
import { formatBytes } from '@/utils/format'
import { formatMemberMoney } from '@/utils/displayCurrency'

defineProps<{ data: PAYGView | null; loading: boolean }>()
const { t } = useI18n()
</script>

<template>
  <div class="payg-summary" :aria-busy="loading">
    <div class="payg-summary__wallet">
      <div><span>{{ t('payg.balance') }}</span><strong v-if="data">{{ formatMemberMoney(data.balance) }}</strong><USkeleton v-else-if="loading" class="h-7 w-28" /><span v-else>{{ t('payg.status.unavailable') }}</span></div>
      <UButton :to="{ path: '/home', query: { topUp: '1' } }" icon="i-ph-plus" color="neutral" variant="outline" :label="t('payg.addBalance')" data-haptic="navigate" />
    </div>
    <div class="payg-summary__window">
      <small>{{ t('payg.window') }}</small>
      <dl v-if="data">
        <div><dt class="sr-only">{{ t('payg.traffic') }}</dt><dd><UIcon name="i-ph-arrows-down-up" aria-hidden="true" />{{ formatBytes(data.trafficBytes) }}</dd></div>
        <div><dt class="sr-only">{{ t('payg.spent') }}</dt><dd><UIcon name="i-ph-coins" aria-hidden="true" />{{ formatMemberMoney(data.spent) }}</dd></div>
      </dl>
      <USkeleton v-else-if="loading" class="h-8 w-44" />
    </div>
  </div>
</template>

<style scoped>
.payg-summary { display: grid; gap: .5rem; padding: 0 0 .65rem; border-bottom: 1px solid var(--line); }
.payg-summary__window { animation: payg-summary 200ms ease-out both; animation-delay: var(--catalog-entry-delay, 0ms); }
.payg-summary__wallet { display: flex; align-items: center; justify-content: space-between; gap: 1rem; }
.payg-summary__wallet > div { display: flex; flex-wrap: wrap; align-items: baseline; min-width: 0; gap: .1rem .6rem; }
.payg-summary__wallet span, .payg-summary__window small, dt { color: var(--text-muted); font-size: .72rem; }
.payg-summary__wallet strong { font-family: var(--font-mono); font-size: 1.35rem; font-weight: 650; overflow-wrap: anywhere; }
.payg-summary__window { display: flex; flex-wrap: wrap; align-items: center; gap: .3rem 1rem; }
.payg-summary__window small { white-space: nowrap; }
.payg-summary dl { display: flex; flex-wrap: wrap; align-items: center; gap: .3rem 1rem; margin: 0; }
.payg-summary dl > div { display: flex; align-items: baseline; flex-wrap: wrap; gap: .55rem; }
.payg-summary dd { display: inline-flex; align-items: center; gap: .3rem; margin: 0; font-family: var(--font-mono); font-size: .85rem; }
.payg-summary dd .iconify { color: var(--text-muted); font-size: .85rem; }
@media (min-width: 900px) { .payg-summary { grid-template-columns: minmax(0, 1fr) auto; align-items: center; gap: 2rem; } .payg-summary__wallet { justify-content: flex-start; } }
@keyframes payg-summary { from { opacity: 0; transform: translateY(4px); } to { opacity: 1; transform: translateY(0); } }
@media (prefers-reduced-motion: reduce) { .payg-summary__window { animation: none; } }
</style>
