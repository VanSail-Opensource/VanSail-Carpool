<script setup lang="ts">
import { computed, shallowRef } from 'vue'

import InlineNotice from '@/components/common/InlineNotice.vue'
import SkeletonBlock from '@/components/common/SkeletonBlock.vue'
import { useStatistics } from '@/composables/useStatistics'
import { useI18n } from '@/i18n'
import { formatDateTime } from '@/utils/format'
import StatisticsDistribution from './StatisticsDistribution.vue'
import StatisticsFreshness from './StatisticsFreshness.vue'
import StatisticsNodes from './StatisticsNodes.vue'
import StatisticsOverview from './StatisticsOverview.vue'
import StatisticsShareCharts from './StatisticsShareCharts.vue'
import StatisticsTrafficChart from './StatisticsTrafficChart.vue'

type StatisticsTab = 'overview' | 'nodes' | 'traffic' | 'distributions' | 'composition'

const { snapshot, nodeSnapshot, loading, refreshing, nodesLoading, error, nodesError, load } = useStatistics()
const { t } = useI18n()
const activeTab = shallowRef<StatisticsTab>('overview')
const tabs = computed(() => [
  { label: t('statistics.overview'), value: 'overview', slot: 'overview' as const, icon: 'i-ph-squares-four' },
  { label: t('statistics.nodes'), value: 'nodes', slot: 'nodes' as const, icon: 'i-ph-hard-drives' },
  { label: t('statistics.sevenDayTraffic'), value: 'traffic', slot: 'traffic' as const, icon: 'i-ph-chart-line-up' },
  { label: t('statistics.distributions'), value: 'distributions', slot: 'distributions' as const, icon: 'i-ph-chart-donut' },
  { label: t('statistics.squadComposition'), value: 'composition', slot: 'composition' as const, icon: 'i-ph-stack' },
])

</script>

<template>
  <div class="admin-panel page--statistics">
    <header class="statistics-heading">
      <div>
        <h1>{{ $t('statistics.title') }}</h1>
        <p v-if="snapshot">{{ $t('statistics.updatedAt', { date: formatDateTime(snapshot.generatedAt) }) }}</p>
      </div>
      <UTooltip :text="$t('statistics.refresh')">
        <UButton
          type="button"
          color="neutral"
          variant="outline"
          square
          class="statistics-heading__refresh"
          icon="i-ph-arrows-clockwise"
          :loading="refreshing"
          :aria-label="$t('statistics.refresh')"
          @click="load({ quiet: true })"
        />
      </UTooltip>
    </header>

    <template v-if="loading && !snapshot">
      <SkeletonBlock height="16rem" />
      <div class="statistics-skeleton-grid"><SkeletonBlock height="13rem" /><SkeletonBlock height="13rem" /></div>
      <SkeletonBlock height="18rem" />
    </template>

    <template v-else-if="snapshot">
      <InlineNotice v-if="snapshot.stalePartitions.length" tone="warning">{{ $t('statistics.stalePartitions') }}</InlineNotice>
      <InlineNotice v-if="error" tone="warning">{{ error }}</InlineNotice>
      <InlineNotice v-if="nodesError" tone="warning">{{ nodesError }}</InlineNotice>
      <UTabs
        v-model="activeTab"
        :items="tabs"
        class="min-w-0 w-full"
        :ui="{ list: 'w-full overflow-x-auto overscroll-x-contain', trigger: 'min-h-11 grow-0 shrink-0 px-3' }"
      >
        <template #overview>
          <StatisticsOverview :snapshot="snapshot" />
          <StatisticsFreshness :snapshot="snapshot" />
        </template>
        <template #nodes>
          <StatisticsNodes :snapshot="nodeSnapshot" :loading="nodesLoading" />
        </template>
        <template #traffic>
          <StatisticsTrafficChart :remote="snapshot.remote" />
        </template>
        <template #distributions>
          <StatisticsShareCharts :database="snapshot.database" />
        </template>
        <template #composition>
          <StatisticsDistribution :database="snapshot.database" />
        </template>
      </UTabs>
    </template>

    <div v-else class="error-state">
      <h1>{{ $t('statistics.unavailable') }}</h1>
      <p>{{ error ?? $t('statistics.loadFailed') }}</p>
      <UButton icon="i-ph-arrows-clockwise" :label="$t('common.tryAgain')" @click="load()" />
    </div>
  </div>
</template>

<style scoped>
.page--statistics { max-width: none; gap: 1rem; }
.statistics-heading h1 { font-size: 1.25rem; }
</style>
