<script setup lang="ts">
import type { PricingRecord } from '@/api/pricingAnalysis'
import { useI18n } from '@/i18n'
import { pricingLabel, pricingMoney } from '@/utils/pricingDisplay'
defineProps<{ title: string; items: PricingRecord[]; fields?: string[] }>()
const { t } = useI18n()
function display(field: string, value: PricingRecord[string]) {
  if (value === null || value === undefined) return t('common.notAvailable')
  if (field.endsWith('_minor')) return pricingMoney(value)
  if (['state', 'status', 'health_state', 'calibration_status'].includes(field)) return pricingLabel('states', value)
  if (field === 'reason') return pricingLabel('reasons', value)
  return value
}
</script>
<template>
  <section class="pricing-records">
    <h3>{{ title }}</h3>
    <p v-if="!items.length" class="text-muted">{{ t('pricing.emptyRecords') }}</p>
    <div v-else class="pricing-records__scroll">
      <dl v-for="(item, index) in items" :key="String(item.id ?? item.forecast_id ?? index)" class="pricing-records__row">
        <template v-for="field in fields ?? Object.keys(item)" :key="field"><dt>{{ t(`pricing.fields.${field}`) === `pricing.fields.${field}` ? field : t(`pricing.fields.${field}`) }}</dt><dd><details v-if="field.endsWith('_json')"><summary>{{ t('pricing.calculationTrace') }}</summary><pre>{{ item[field] }}</pre></details><template v-else>{{ display(field, item[field]) }}</template></dd></template>
      </dl>
    </div>
  </section>
</template>
<style scoped>
.pricing-records { min-width: 0; }
.pricing-records h3 { margin: 0 0 1rem; font-size: 1rem; }
.pricing-records__scroll { overflow-x: auto; }
.pricing-records__row { display: grid; grid-template-columns: minmax(8rem, 1fr) minmax(0, 3fr); gap: .5rem 1rem; margin: 0; padding: 1rem 0; border-top: 1px solid var(--line); font-size: .8rem; }
.pricing-records dt { color: var(--text-muted); overflow-wrap: anywhere; }
.pricing-records dd, .pricing-records pre { min-width: 0; margin: 0; white-space: pre-wrap; overflow-wrap: anywhere; font: inherit; }
@media (max-width: 520px) { .pricing-records__row { grid-template-columns: 1fr; gap: .25rem; } .pricing-records dd { margin-bottom: .5rem; } }
</style>
