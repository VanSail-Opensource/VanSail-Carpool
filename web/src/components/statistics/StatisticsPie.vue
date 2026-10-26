<script setup lang="ts">
import { computed } from 'vue'
import { VisDonut, VisSingleContainer } from '@unovis/vue'
import { Donut } from '@unovis/ts'
import { ChartContainer } from '@/components/charts'

import type { NamedShare } from '@/api/types'
import StatisticsChartDetail from './StatisticsChartDetail.vue'
import { chartSegments, formatStatisticNumber, formatStatisticPercent } from './statisticsFormat'
import { useStatisticsChartSelection } from './useStatisticsChartSelection'

const props = withDefaults(defineProps<{
  items: readonly NamedShare[]
  chartLabel: string
  sliceLabels?: boolean
}>(), { sliceLabels: false })

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
  <div class="statistics-pie-layout" :class="{ 'statistics-pie-layout--labels': sliceLabels }">
    <ChartContainer v-if="segments.length" :config="{}" :aria-label="chartLabel"><VisSingleContainer :data="segments" :height="220" :duration="0"><VisDonut :value="(d: typeof segments[number]) => Number(d.value)" :color="(d: typeof segments[number]) => d.color" :arc-width="0" :events="donutEvents" /></VisSingleContainer></ChartContainer>
    <p v-else class="statistics-empty">{{ $t('statistics.noData') }}</p>
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
  </div>
</template>
