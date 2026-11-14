<script setup lang="ts">
import { computed } from 'vue'
import { t } from '@/i18n'
import { isBrowserExperience } from '@/utils/experience'
import { selectionHaptic } from '@/utils/telegramHaptics'
import type { UsageBreakdownMetric } from './usageBreakdownSegments'

const metric = defineModel<UsageBreakdownMetric>({ required: true })
const tabs = computed(() => [
  { label: t('usageBreakdown.traffic'), value: 'traffic', icon: 'i-ph-arrows-down-up' },
  { label: t('usageBreakdown.cost'), value: 'cost', icon: 'i-ph-coins' },
])
function select(value: string | number): void {
  if ((value !== 'traffic' && value !== 'cost') || value === metric.value) return
  metric.value = value
  if (!isBrowserExperience()) selectionHaptic()
}
</script>

<template>
  <UTabs :model-value="metric" :items="tabs" :content="false" color="neutral" variant="pill" size="sm" :ui="{ root: 'shrink-0', trigger: 'min-h-11 gap-1 px-2 sm:px-3', leadingIcon: 'size-3.5' }" :aria-label="t('usageBreakdown.measure')" @update:model-value="select" />
</template>
