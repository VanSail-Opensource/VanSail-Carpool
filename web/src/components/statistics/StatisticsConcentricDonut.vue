<script setup lang="ts">
import { computed } from 'vue'
import { motion } from 'motion-v'

import type { NamedShare } from '@/api/types'
import { motionDurations } from '@/composables/motionPresets'
import { useMotionPreferences } from '@/composables/useMotionPreferences'
import { ringSegments } from './statisticsGeometry'
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
const { activeItem, hasActive, activate, deactivate, select, isActive, isSelected } = useStatisticsChartSelection(allSegments)
const { reducedMotion } = useMotionPreferences()
const ringRadius = (index: number) => 50 - index * (18 / Math.max(1, chartRings.value.length - 1))
const manyRings = computed(() => chartRings.value.length > 2)
</script>

<template>
  <div v-if="hasSegments" class="statistics-concentric-layout">
    <div class="statistics-concentric-rings" :class="{ 'statistics-concentric-rings--many': manyRings }">
      <svg viewBox="0 0 120 120" role="group" :aria-label="chartLabel">
        <title>{{ chartLabel }}</title>
        <template v-for="(ring, index) in chartRings" :key="ring.id">
          <motion.circle
            class="statistics-ring-track"
            :class="index === 0 ? 'statistics-concentric-ring--outer' : 'statistics-concentric-ring--inner'"
            cx="60"
            cy="60"
            :r="ringRadius(index)"
            pathLength="100"
          />
          <motion.circle
            v-for="segment in ringSegments(ring.segments)"
            :key="segment.interactionId"
            class="statistics-ring-segment"
            :class="[
              'statistics-chart-segment',
              index === 0 ? 'statistics-concentric-ring--outer' : 'statistics-concentric-ring--inner',
              { 'statistics-chart-segment--muted': hasActive && !isActive(segment.interactionId) },
            ]"
            cx="60"
            cy="60"
            :r="ringRadius(index)"
            pathLength="100"
            :stroke="segment.color"
            :initial="reducedMotion ? { opacity: 0, strokeDasharray: segment.dasharray, strokeDashoffset: segment.dashoffset } : { opacity: 0, strokeDasharray: '0 100' }"
            :animate="{ opacity: 1, strokeDasharray: segment.dasharray, strokeDashoffset: segment.dashoffset }"
            :transition="{ duration: reducedMotion ? 0.08 : motionDurations.data, delay: reducedMotion ? 0 : index * 0.03, ease: 'easeOut' }"
            role="button"
            tabindex="0"
            :aria-label="$t('statistics.chartSeries', { series: segment.ringLabel, segment: $t('statistics.chartSeries', { series: segment.label, segment: $t('statistics.chartPointValue', { value: formatStatisticNumber(segment.value), percent: formatStatisticPercent(segment.percentage) }) }) })"
            :aria-pressed="isSelected(segment.interactionId)"
            @pointerenter="activate(segment.interactionId)"
            @pointerleave="deactivate(segment.interactionId)"
            @focus="activate(segment.interactionId)"
            @blur="deactivate(segment.interactionId)"
            @click="select(segment.interactionId)"
            @keydown.enter.prevent="select(segment.interactionId)"
            @keydown.space.prevent="select(segment.interactionId)"
          />
        </template>
      </svg>
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
