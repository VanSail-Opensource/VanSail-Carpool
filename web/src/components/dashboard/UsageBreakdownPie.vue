<script setup lang="ts">
import { computed, shallowRef, watch } from 'vue'
import { VisDonut, VisSingleContainer } from '@unovis/vue'
import { Donut } from '@unovis/ts'
import type { UsageBreakdown } from '@/api/usageBreakdown'
import { ChartContainer } from '@/components/charts'
import { useStatisticsChartSelection } from '@/components/statistics/useStatisticsChartSelection'
import { formatStatisticPercent } from '@/components/statistics/statisticsFormat'
import { t } from '@/i18n'
import { formatBytes, formatMoney } from '@/utils/format'
import { isBrowserExperience } from '@/utils/experience'
import { selectionHaptic } from '@/utils/telegramHaptics'
import { formatUsageCost } from './usageBreakdownFormat'
import { readUsageAmount } from '@/utils/usageRational'
import { assignUsageEntryColors, usageEntryIDs, usageBreakdownSegments, usageSegmentColor, usageSegmentID, usageSegmentValue, type UsageBreakdownMetric, type UsageBreakdownSegment } from './usageBreakdownSegments'

const props = defineProps<{ history: UsageBreakdown }>()
const metric = shallowRef<UsageBreakdownMetric>('traffic')
const nodeColors = shallowRef(new Map<string, string>())
watch(() => usageEntryIDs(props.history), ids => { nodeColors.value = assignUsageEntryColors(ids, nodeColors.value) }, { immediate: true })
const plotted = computed(() => usageBreakdownSegments(props.history, metric.value, nodeColors.value))
const segments = computed(() => plotted.value.segments.map(segment => ({ ...segment,
  name: segment.kind === 'node' || segment.kind === 'hourly' ? segment.name || t(segment.kind === 'hourly' ? 'usageBreakdown.unnamedHourlySquad' : 'usageBreakdown.unnamedNode') : t(`usageBreakdown.${segment.kind}`),
})))
const tabs = computed(() => [
  { label: t('usageBreakdown.traffic'), value: 'traffic' },
  { label: t('usageBreakdown.cost'), value: 'cost' },
])
const total = computed(() => metric.value === 'traffic' ? formatBytes(plotted.value.total.numerator.toString()) : formatUsageCost(plotted.value.total))
const emptyMessage = computed(() => plotted.value.total.numerator === 0n
  ? t(metric.value === 'traffic' ? 'usageBreakdown.emptyTraffic' : 'usageBreakdown.emptyCost')
  : t('usageBreakdown.noAttribution'))
const actualCost = computed(() => formatMoney({ currency: 'TXB', minor: props.history.cost.actualMinor, display: '' }))
const legacyCost = computed(() => props.history.cost.legacy.amountMinor ? formatUsageCost(readUsageAmount(props.history.cost.legacy.amountMinor)) : '')
const { activeItem, activate, deactivate, select, isSelected } = useStatisticsChartSelection(segments)
const config = {}
function feedback(): void { if (!isBrowserExperience()) selectionHaptic() }
function selectMetric(value: string | number): void {
  if ((value !== 'traffic' && value !== 'cost') || value === metric.value) return
  metric.value = value
  feedback()
}
function selectSegment(id: string): void { select(id); feedback() }
function percentage(segment: UsageBreakdownSegment): string {
  return segment.amount.numerator > 0n && segment.percentage < 0.1 ? t('usageBreakdown.lessThanPercent', { amount: formatStatisticPercent(0.1) }) : formatStatisticPercent(segment.percentage)
}
const events = { [Donut.selectors.segment]: {
  pointerenter: (d: { data: typeof segments.value[number] }) => activate(d.data.interactionId),
  pointerleave: (d: { data: typeof segments.value[number] }) => deactivate(d.data.interactionId),
  click: (d: { data: typeof segments.value[number] }) => selectSegment(d.data.interactionId),
} }
</script>

<template>
  <div class="usage-breakdown" data-usage-breakdown>
    <UTabs :model-value="metric" :items="tabs" :content="false" color="neutral" variant="pill" size="sm" :ui="{ root: 'justify-self-start', trigger: 'min-h-11 px-4' }" :aria-label="t('usageBreakdown.measure')" @update:model-value="selectMetric" />
    <dl class="flex items-baseline justify-between gap-3 text-sm"><dt class="min-w-0 text-muted">{{ t(metric === 'traffic' ? 'usageBreakdown.trafficTotal' : 'usageBreakdown.costTotal') }}</dt><dd class="min-w-0 max-w-[70%] break-all text-right font-semibold tabular-nums">{{ total }}</dd></dl>
    <div v-if="metric === 'cost'" class="usage-breakdown__notes grid gap-1 text-xs text-muted" data-usage-cost-notes>
      <p>{{ t('usageBreakdown.actualCost', { amount: actualCost }) }}</p>
      <p v-if="history.cost.legacy.status === 'available'">{{ t('usageBreakdown.legacyEstimate', { amount: legacyCost }) }}</p>
      <p v-else-if="history.cost.legacy.status === 'original_term_unknown'">{{ t('usageBreakdown.legacyUnknown') }}</p>
      <p v-if="history.cost.unallocatedMinor.numerator !== '0'">{{ t('usageBreakdown.unallocatedHint') }}</p>
    </div>
    <p v-if="!plotted.showPie" role="status" class="py-4 text-sm text-muted" data-usage-no-comparison>{{ emptyMessage }}</p>
    <div v-show="plotted.showPie" class="usage-breakdown__body" data-usage-chart>
      <ChartContainer role="img" :config="config" :aria-label="t(metric === 'traffic' ? 'usageBreakdown.trafficChart' : 'usageBreakdown.costChart')" class="usage-breakdown__chart">
        <VisSingleContainer :data="segments" :height="220" :duration="0"><VisDonut :id="usageSegmentID" :value="usageSegmentValue" :color="usageSegmentColor" :duration="0" :arc-width="0" :events="events" /></VisSingleContainer>
      </ChartContainer>
      <ul class="grid gap-1 min-w-0 content-start">
        <li v-for="segment in segments" :key="segment.interactionId">
          <UButton type="button" color="neutral" variant="ghost" class="usage-breakdown__legend" :class="{ 'usage-breakdown__legend--active': activeItem?.interactionId === segment.interactionId }" :aria-pressed="isSelected(segment.interactionId)" @pointerenter="activate(segment.interactionId)" @pointerleave="deactivate(segment.interactionId)" @focus="activate(segment.interactionId)" @blur="deactivate(segment.interactionId)" @click="selectSegment(segment.interactionId)">
            <span class="h-2 w-2 rounded-sm shrink-0" :style="{ background: segment.color }" aria-hidden="true" />
            <span class="min-w-0 break-words"><span>{{ segment.name }}</span><span v-if="segment.kind === 'hourly'" class="block text-xs text-muted">{{ t('usageBreakdown.hourlySquad') }}</span></span>
            <span class="ml-auto whitespace-nowrap text-muted tabular-nums">{{ percentage(segment) }}</span>
          </UButton>
        </li>
      </ul>
    </div>
    <output v-if="plotted.showPie && activeItem" class="sr-only" aria-live="polite">{{ activeItem.name }}: {{ percentage(activeItem) }}</output>
  </div>
</template>

<style scoped>
.usage-breakdown { display: grid; gap: .6rem; min-width: 0; }
.usage-breakdown__body { display: grid; grid-template-columns: minmax(0,1fr); gap: 1rem; }
.usage-breakdown__chart { height: 220px; }
.usage-breakdown__notes { overflow-wrap: anywhere; }
.usage-breakdown__legend { display: flex; align-items: center; width: 100%; text-align: left; gap: .5rem; padding: .5rem; min-height: 44px; font-size: .8rem; border-radius: .35rem; }
.usage-breakdown__legend--active, .usage-breakdown__legend[aria-pressed="true"] { background: var(--surface); }
.usage-breakdown__legend:focus-visible { outline: 2px solid var(--accent); outline-offset: 2px; }
@media(min-width:600px) { .usage-breakdown__body { grid-template-columns: minmax(0,1fr) minmax(0,1fr); align-items: start; } }
</style>
