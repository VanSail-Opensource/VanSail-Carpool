<script setup lang="ts">
import { computed, shallowRef } from 'vue'
import { motion } from 'motion-v'

import type { NormalizedDistribution, StatisticsSnapshot } from '@/api/types'
import { motionDurations } from '@/composables/motionPresets'
import { useMotionPreferences } from '@/composables/useMotionPreferences'
import { useI18n } from '@/i18n'
import StatisticsChartDetail from './StatisticsChartDetail.vue'
import { statisticsColors, formatStatisticNumber, formatStatisticPercent } from './statisticsFormat'
import { useStatisticsChartSelection } from './useStatisticsChartSelection'

const props = defineProps<{ database: StatisticsSnapshot['database'] }>()
const { t } = useI18n()
const mode = shallowRef<'squad' | 'combo'>('squad')
const tabs = computed(() => [
  { label: t('statistics.bySquad'), value: 'squad' },
  { label: t('statistics.byCombo'), value: 'combo' },
])
const rows = computed<readonly NormalizedDistribution[]>(() => mode.value === 'squad'
  ? props.database.comboBySquad
  : props.database.squadByCombo)
const legend = computed(() => {
  const values = new Map<string, string>()
  for (const row of rows.value) for (const segment of row.segments) values.set(segment.id, segment.label)
  return [...values].map(([id, label], index) => ({ id, label, color: statisticsColors[index % statisticsColors.length] }))
})
const colors = computed(() => new Map(legend.value.map((item) => [item.id, item.color])))
const barRows = computed(() => rows.value.map((row) => {
  const total = row.segments.reduce((sum, segment) => sum + Math.max(0, segment.value), 0)
  let cursor = 0
  const segments = row.segments.filter((segment) => segment.value > 0).map((segment) => {
    const width = total > 0 ? segment.value * 100 / total : 0
    const result = {
      ...segment,
      interactionId: `${row.id}:${segment.id}`,
      groupLabel: row.label,
      x: cursor,
      width,
      color: colors.value.get(segment.id) ?? statisticsColors[0],
    }
    cursor += width
    return result
  })
  return { ...row, segments }
}))
const allSegments = computed(() => barRows.value.flatMap((row) => row.segments))
const { activeItem, hasActive, activate, deactivate, select, isActive, isSelected } = useStatisticsChartSelection(allSegments)
const { reducedMotion } = useMotionPreferences()

</script>

<template>
  <section class="statistics-section">
    <div class="statistics-distribution-heading">
      <h2>{{ $t('statistics.squadComposition') }}</h2>
      <UTabs v-model="mode" :items="tabs" :content="false" size="sm" />
    </div>
    <article v-if="barRows.length" class="statistics-panel statistics-distribution">
      <div v-for="row in barRows" :key="row.id" class="statistics-distribution__row">
        <UTooltip :text="row.label"><strong>{{ row.label }}</strong></UTooltip>
        <svg
          class="statistics-distribution__bar"
          viewBox="0 0 100 20"
          preserveAspectRatio="none"
          role="group"
          :aria-label="$t('statistics.compositionLabel', { name: row.label })"
        >
          <rect class="statistics-distribution__track" x="0" y="0" width="100" height="20" aria-hidden="true" />
          <motion.rect
            v-for="segment in row.segments"
            :key="segment.id"
            class="statistics-distribution__motion"
            :class="{ 'statistics-chart-segment--muted': hasActive && !isActive(segment.interactionId) }"
            y="0"
            height="20"
            :fill="segment.color"
            :initial="false"
            :animate="{ x: segment.x, width: segment.width }"
            :transition="{ duration: reducedMotion ? 0.08 : motionDurations.data, ease: 'easeOut' }"
            aria-hidden="true"
          />
          <rect
            v-for="segment in row.segments"
            :key="`interaction-${segment.id}`"
            class="statistics-distribution__segment statistics-chart-segment"
            :class="{ 'statistics-chart-segment--muted': hasActive && !isActive(segment.interactionId) }"
            :x="segment.x"
            y="0"
            :width="segment.width"
            height="20"
            fill="transparent"
            role="button"
            :aria-label="$t('statistics.compositionPoint', { group: row.label, segment: segment.label, value: formatStatisticPercent(segment.width) })"
            :aria-pressed="isSelected(segment.interactionId)"
            tabindex="0"
            @pointerenter="activate(segment.interactionId)"
            @pointerleave="deactivate(segment.interactionId)"
            @focus="activate(segment.interactionId)"
            @blur="deactivate(segment.interactionId)"
            @click="select(segment.interactionId)"
            @keydown.enter.prevent="select(segment.interactionId)"
            @keydown.space.prevent="select(segment.interactionId)"
          />
        </svg>
      </div>
      <StatisticsChartDetail
        v-if="activeItem"
        :color="activeItem.color"
        :label="$t('statistics.chartSeries', { series: activeItem.groupLabel, segment: activeItem.label })"
        :value="$t('statistics.chartPointValue', { value: formatStatisticNumber(activeItem.value), percent: formatStatisticPercent(activeItem.width) })"
      />
      <ul class="statistics-distribution__legend">
        <li v-for="item in legend" :key="item.id"><span class="statistics-legend__swatch" :style="{ backgroundColor: item.color }" /><span :title="item.label">{{ item.label }}</span></li>
      </ul>
    </article>
    <div v-else class="statistics-empty statistics-empty--panel"><UIcon name="i-ph-stack" aria-hidden="true" /><span>{{ $t('statistics.noDistribution') }}</span></div>
  </section>
</template>

<style scoped>
.statistics-distribution__motion { pointer-events: none; }
</style>
