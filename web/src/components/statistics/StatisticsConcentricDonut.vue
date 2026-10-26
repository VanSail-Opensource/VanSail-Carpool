<script setup lang="ts">
import { computed } from 'vue'
import { VisDonut, VisSingleContainer } from '@unovis/vue'
import { Donut } from '@unovis/ts'
import { ChartContainer } from '@/components/charts'

import type { NamedShare } from '@/api/types'
import StatisticsChartDetail from './StatisticsChartDetail.vue'
import { chartSegments, formatStatisticNumber, formatStatisticPercent } from './statisticsFormat'
import { useStatisticsChartSelection } from './useStatisticsChartSelection'

interface ConcentricRing {
  id: string
  label: string
  items: readonly NamedShare[]
  /** Share of the full ring reserved for this series. Defaults to a full circle. */
  weight?: number
}

const props = defineProps<{
  rings: readonly ConcentricRing[]
  centerLabel: string
  centerValue: string
  chartLabel: string
  centerIcon?: string
}>()

const chartRings = computed(() => props.rings.map((ring) => ({
  ...ring,
  segments: chartSegments(ring.items, ring.weight ?? 100).map((segment) => ({
    ...segment,
    interactionId: `${ring.id}:${segment.id}`,
    ringLabel: ring.label,
  })),
})))
const hasSegments = computed(() => chartRings.value.some((ring) => ring.segments.length > 0))
const allSegments = computed(() => chartRings.value.flatMap((ring) => ring.segments))
const { activeItem, activate, deactivate, select, isSelected } = useStatisticsChartSelection(allSegments)
const manyRings = computed(() => chartRings.value.length > 2)
const donutEvents = { [Donut.selectors.segment]: {
 pointerenter: (d: { data: { interactionId: string } }) => activate(d.data.interactionId),
 pointerleave: (d: { data: { interactionId: string } }) => deactivate(d.data.interactionId),
 click: (d: { data: { interactionId: string } }) => select(d.data.interactionId),
} }
</script>

<template>
  <div v-if="hasSegments" class="statistics-concentric-layout">
    <div class="statistics-concentric-rings" :class="{ 'statistics-concentric-rings--many': manyRings }">
      <ChartContainer :config="{}" class="concentric-unovis" :aria-label="chartLabel"><div v-for="(ring, index) in chartRings" :key="ring.id" class="concentric-unovis__ring"><VisSingleContainer :data="ring.segments" :height="240" :duration="0"><VisDonut :events="donutEvents" :value="(d: typeof ring.segments[number]) => Number(d.value)" :color="(d: typeof ring.segments[number]) => d.color" :radius="104 - index * (40 / Math.max(1, chartRings.length - 1))" :arc-width="Math.min(22, 48 / chartRings.length)" :pad-angle=".01" :angle-range="[0, Math.PI * 2 * ((ring.weight ?? 100) / 100)]" /></VisSingleContainer></div></ChartContainer>
      <UIcon v-if="centerIcon" class="statistics-concentric__icon" :name="centerIcon" aria-hidden="true" />
      <div v-else class="statistics-concentric__center" aria-hidden="true">
        <strong>{{ centerValue }}</strong>
        <span>{{ centerLabel }}</span>
      </div>
    </div>
    <div class="statistics-concentric-legends">
      <div v-for="ring in chartRings" :key="ring.id">
        <strong>{{ ring.label }}</strong>
        <ul v-if="ring.segments.length" class="statistics-legend">
          <li
            v-for="segment in ring.segments"
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
      </div>
    </div>
    <StatisticsChartDetail
      v-if="activeItem"
      :color="activeItem.color"
      :label="$t('statistics.chartSeries', { series: activeItem.ringLabel, segment: activeItem.label })"
      :value="$t('statistics.chartPointValue', { value: formatStatisticNumber(activeItem.value), percent: formatStatisticPercent(activeItem.percentage) })"
    />
  </div>
</template>

<style scoped>
.statistics-concentric-rings { position: relative; width: 240px; max-width: 100%; min-height: 240px; display: block; }
.statistics-concentric__center, .statistics-concentric__icon { position: absolute; left: 50%; top: 50%; transform: translate(-50%, -50%); width: 30%; }
.concentric-unovis { position: relative; height: 240px; }.concentric-unovis__ring { position: absolute; inset: 0; pointer-events: none; }
.concentric-unovis__ring :deep(.vis-donut-segment) { pointer-events: auto; }
</style>
