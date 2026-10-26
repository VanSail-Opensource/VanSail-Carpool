<script setup lang="ts">
import { computed } from 'vue'
import { AnimatePresence, motion } from 'motion-v'

import type { NamedShare, StatisticsSnapshot } from '@/api/types'
import { useMotionPreferences } from '@/composables/useMotionPreferences'
import { useI18n } from '@/i18n'
import { formatBytes } from '@/utils/format'
import { formatMemberMoney } from '@/utils/displayCurrency'
import StatisticsDonut from './StatisticsDonut.vue'
import { formatSignedStatistic, formatStatisticNumber, formatStatisticPercent } from './statisticsFormat'

const props = defineProps<{ snapshot: StatisticsSnapshot }>()
const { t } = useI18n()
const { reducedMotion } = useMotionPreferences()

const conversion = computed(() => Math.min(100, Math.max(0, props.snapshot.database.newUserConversionPercent)))
const conversionItems = computed<NamedShare[]>(() => [
  { id: 'converted', label: t('statistics.converted'), value: conversion.value },
  { id: 'unconverted', label: t('statistics.notConverted'), value: 100 - conversion.value },
])
const metrics = computed(() => {
  const database = props.snapshot.database
  return [
    { id: 'usage', icon: 'i-ph-gauge', label: t('statistics.monthlyUsage'), value: formatStatisticPercent(props.snapshot.remote.monthlyAverageUsagePercent) },
    { id: 'spend', icon: 'i-ph-coins', label: t('statistics.averageSpend'), value: formatMemberMoney(database.averageSpend) },
    { id: 'range', icon: 'i-ph-arrows-left-right', label: t('statistics.spendRange'), value: t('statistics.valueRange', { minimum: formatMemberMoney(database.spendMinimum), maximum: formatMemberMoney(database.spendMaximum) }) },
    { id: 'checkin', icon: 'i-ph-gift', label: t('statistics.averageCheckIn'), value: formatMemberMoney(database.averageCheckInReward) },
    { id: 'messages', icon: 'i-ph-chat-circle-dots', label: t('statistics.groupMessages'), value: formatStatisticNumber(database.groupMessagesTotal) },
    { id: 'squads', icon: 'i-ph-stack', label: t('statistics.optionalSquads'), value: formatStatisticNumber(database.averageOptionalSquads, 2) },
    { id: 'database', icon: 'i-ph-database', label: t('statistics.databaseSize'), value: formatBytes(database.databaseBytes) },
    { id: 'combos', icon: 'i-ph-package', label: t('statistics.coreCombosRepresented'), value: formatStatisticNumber(database.squadByCombo.length) },
    { id: 'composition', icon: 'i-ph-tree-structure', label: t('statistics.internalSquadsRepresented'), value: formatStatisticNumber(database.comboBySquad.length) },
  ]
})
</script>

<template>
  <section class="statistics-section">
    <div class="statistics-section__heading"><h2>{{ $t('statistics.overview') }}</h2></div>
    <div class="statistics-overview">
      <article class="statistics-panel statistics-conversion">
        <StatisticsDonut
          :items="conversionItems"
          :center-label="$t('statistics.weeklyIncrease')"
          :center-value="formatSignedStatistic(snapshot.remote.weeklyUserIncrease)"
          :chart-label="$t('statistics.weeklyIncreaseChartLabel', { value: formatSignedStatistic(snapshot.remote.weeklyUserIncrease) })"
          legend-below
        />
      </article>
      <article class="statistics-panel statistics-metrics">
        <dl class="statistics-metrics__grid">
          <div v-for="metric in metrics" :key="metric.id">
            <dt><UIcon :name="metric.icon" aria-hidden="true" />{{ metric.label }}</dt>
            <dd :title="metric.value">
              <AnimatePresence mode="wait" :initial="false">
                <motion.output
                  :key="`${metric.id}:${metric.value}`"
                  :initial="reducedMotion ? { opacity: 0 } : { opacity: 0, y: 3 }"
                  :animate="{ opacity: 1, y: 0 }"
                  :exit="{ opacity: 0 }"
                  :transition="{ duration: reducedMotion ? 0.08 : 0.2, ease: 'easeOut' }"
                >
                  {{ metric.value }}
                </motion.output>
              </AnimatePresence>
            </dd>
          </div>
        </dl>
      </article>
    </div>
  </section>
</template>
