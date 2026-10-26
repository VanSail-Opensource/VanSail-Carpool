<script setup lang="ts">
import { computed } from 'vue'
import { VisDonut, VisSingleContainer } from '@unovis/vue'
import { Donut } from '@unovis/ts'
import { ChartContainer } from '@/components/charts'

import type { NamedShare } from '@/api/types'
import StatisticsChartDetail from './StatisticsChartDetail.vue'
import { chartSegments, formatStatisticNumber, formatStatisticPercent } from './statisticsFormat'
import { useStatisticsChartSelection } from './useStatisticsChartSelection'

const props = defineProps<{
  items: readonly NamedShare[]
  centerLabel: string
  centerValue: string
  chartLabel: string
  legendBelow?: boolean
}>()

const segments = computed(() => chartSegments(props.items).map((segment) => ({
  ...segment,
  interactionId: segment.id,
})))
const { activeItem, activate, deactivate, select, isSelected } = useStatisticsChartSelection(segments)
const donutEvents = { [Donut.selectors.segment]: {
 pointerenter: (d: { data: { interactionId: string } }) => activate(d.data.interactionId),
 pointerleave: (d: { data: { interactionId: string } }) => deactivate(d.data.interactionId),
 click: (d: { data: { interactionId: string } }) => select(d.data.interactionId),
} }
</script>

<template>
  <div class="statistics-donut-layout" :class="{ 'statistics-donut-layout--legend-below': legendBelow }">
    <div class="statistics-donut">
      <ChartContainer :config="{}" role="group" :aria-label="chartLabel"><VisSingleContainer :data="segments" :height="220" :duration="0"><VisDonut :events="donutEvents" :value="(d: typeof segments[number]) => Number(d.value)" :color="(d: typeof segments[number]) => d.color" :arc-width="24" :central-label="centerValue" :central-sub-label="centerLabel" :pad-angle=".015" /></VisSingleContainer></ChartContainer>
    </div>
    <ul v-if="segments.length" class="statistics-legend">
      <li
        v-for="segment in segments"
        :key="segment.id"
        class="statistics-chart-legend-item"
        role="button"
        tabindex="0"
        :aria-pressed="isSelected(segment.interactionId)"
        @pointerenter="activate(segment.interactionId)"
        @pointerleave="deactivate(segment.interactionId)"
        @focus="activate(segment.interactionId)"
        @blur="deactivate(segment.interactionId)"
        @click="select(segment.interactionId)"
        @keydown.enter.prevent="select(segment.interactionId)"
        @keydown.space.prevent="select(segment.interactionId)"
      >
        <span class="statistics-legend__swatch" :style="{ backgroundColor: segment.color }" />
        <span class="statistics-legend__label" :title="segment.label">{{ segment.label }}</span>
        <strong>{{ formatStatisticPercent(segment.percentage) }}</strong>
      </li>
    </ul>
    <StatisticsChartDetail
      v-if="activeItem"
      :color="activeItem.color"
      :label="activeItem.label"
      :value="$t('statistics.chartPointValue', { value: formatStatisticNumber(activeItem.value), percent: formatStatisticPercent(activeItem.percentage) })"
    />
    <p v-else class="statistics-empty">{{ $t(segments.length ? 'charts.inspect' : 'statistics.noData') }}</p>
  </div>
</template>
