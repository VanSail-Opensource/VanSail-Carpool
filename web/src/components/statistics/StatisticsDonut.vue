<script setup lang="ts">
import { computed } from 'vue'
import { motion } from 'motion-v'

import type { NamedShare } from '@/api/types'
import { motionDurations } from '@/composables/motionPresets'
import { useMotionPreferences } from '@/composables/useMotionPreferences'
import StatisticsChartDetail from './StatisticsChartDetail.vue'
import { ringSegments } from './statisticsGeometry'
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
const rings = computed(() => ringSegments(segments.value))
const { activeItem, hasActive, activate, deactivate, select, isActive, isSelected } = useStatisticsChartSelection(segments)
const { reducedMotion } = useMotionPreferences()
</script>

<template>
  <div class="statistics-donut-layout" :class="{ 'statistics-donut-layout--legend-below': legendBelow }">
    <div class="statistics-donut">
      <svg viewBox="0 0 120 120" role="group" :aria-label="chartLabel">
        <title>{{ chartLabel }}</title>
        <circle class="statistics-ring-track" cx="60" cy="60" r="47" pathLength="100" />
        <motion.circle
          v-for="ring in rings"
          :key="ring.id"
          class="statistics-ring-segment"
          :class="['statistics-chart-segment', { 'statistics-chart-segment--muted': hasActive && !isActive(ring.interactionId) }]"
          cx="60"
          cy="60"
          r="47"
          pathLength="100"
          :stroke="ring.color"
          :initial="reducedMotion ? { opacity: 0, strokeDasharray: ring.dasharray, strokeDashoffset: ring.dashoffset } : { opacity: 0, strokeDasharray: '0 100' }"
          :animate="{ opacity: 1, strokeDasharray: ring.dasharray, strokeDashoffset: ring.dashoffset }"
          :transition="{ duration: reducedMotion ? 0.08 : motionDurations.data, ease: 'easeOut' }"
          role="button"
          tabindex="0"
          :aria-label="$t('statistics.chartSeries', { series: ring.label, segment: $t('statistics.chartPointValue', { value: formatStatisticNumber(ring.value), percent: formatStatisticPercent(ring.percentage) }) })"
          :aria-pressed="isSelected(ring.interactionId)"
          @pointerenter="activate(ring.interactionId)"
          @pointerleave="deactivate(ring.interactionId)"
          @focus="activate(ring.interactionId)"
          @blur="deactivate(ring.interactionId)"
          @click="select(ring.interactionId)"
          @keydown.enter.prevent="select(ring.interactionId)"
          @keydown.space.prevent="select(ring.interactionId)"
        />
      </svg>
      <div class="statistics-donut__center" aria-hidden="true">
        <strong>{{ centerValue }}</strong>
        <span>{{ centerLabel }}</span>
      </div>
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
    <p v-else class="statistics-empty">{{ $t('statistics.noData') }}</p>
  </div>
</template>
