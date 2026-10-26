<script setup lang="ts">
import type { components } from '@/api/generated'
import type { DeepReadonly } from '@/api/types'
import { useI18n } from '@/i18n'
import { formatDate } from '@/utils/format'
defineProps<{ decision: DeepReadonly<components['schemas']['PricingDecision']> }>()
const { t } = useI18n()
</script>
<template>
  <dl class="decision-facts">
    <div><dt>{{ t('userPrices.status') }}</dt><dd>{{ t(`userPrices.statuses.${decision.status}`) }}</dd></div>
    <div><dt>{{ t('pricing.reason') }}</dt><dd><code>{{ decision.reason }}</code></dd></div>
    <div><dt>{{ t('userPrices.probability') }}</dt><dd>{{ decision.probability === undefined ? t('common.notAvailable') : `${(decision.probability * 100).toFixed(1)}%` }}</dd></div>
    <div><dt>{{ t('predictions.calibration') }}</dt><dd>{{ decision.calibration || t('predictions.unvalidated') }}</dd></div>
    <div><dt>{{ t('predictions.freshness') }}</dt><dd>{{ formatDate(decision.generatedAt) }}</dd></div>
    <div><dt>{{ t('userPrices.validUntil') }}</dt><dd>{{ formatDate(decision.validUntil) }}</dd></div>
    <div><dt>{{ t('userPrices.version') }}</dt><dd><code>{{ decision.version }}</code></dd></div>
    <div><dt>{{ t('pricing.modelVersion') }}</dt><dd><code>{{ decision.modelVersion || t('common.notAvailable') }}</code></dd></div>
  </dl>
</template>
<style scoped>.decision-facts { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, 12rem), 1fr)); gap: .75rem 1rem; margin: 0; }.decision-facts dt { color: var(--text-muted); font-size: .75rem; }.decision-facts dd { margin: .2rem 0 0; font-size: .8rem; overflow-wrap: anywhere; }</style>
