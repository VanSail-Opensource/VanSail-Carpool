<script setup lang="ts">
import { computed, shallowRef, watch } from 'vue'
import { VisDonut, VisSingleContainer } from '@unovis/vue'
import { Donut } from '@unovis/ts'
import type { PAYGNodeHistory } from '@/api/payg'
import { ChartContainer } from '@/components/charts'
import { useStatisticsChartSelection } from '@/components/statistics/useStatisticsChartSelection'
import { formatStatisticPercent } from '@/components/statistics/statisticsFormat'
import { t } from '@/i18n'
import { paygHistorySegments, type PAYGHistoryMetric } from './paygHistorySegments'

const props = defineProps<{ history: PAYGNodeHistory }>()
const metric = shallowRef<PAYGHistoryMetric>('traffic')
watch(() => props.history.costAvailable, available => { if (!available) metric.value = 'traffic' })
const plotted = computed(() => paygHistorySegments(props.history, metric.value))
const segments = computed(() => plotted.value.segments)
const { activeItem, activate, deactivate, select, isSelected } = useStatisticsChartSelection(segments)
const events = { [Donut.selectors.segment]: {
  pointerenter: (d: { data: typeof segments.value[number] }) => activate(d.data.interactionId),
  pointerleave: (d: { data: typeof segments.value[number] }) => deactivate(d.data.interactionId),
  click: (d: { data: typeof segments.value[number] }) => select(d.data.interactionId),
} }
</script>

<template>
  <div class="payg-usage-pie">
    <div class="flex gap-1 justify-self-start" role="group" :aria-label="t('paygHistory.measure')">
      <UButton size="sm" color="neutral" :variant="metric === 'traffic' ? 'soft' : 'ghost'" :aria-pressed="metric === 'traffic'" :label="t('paygHistory.trafficShare')" @click="metric = 'traffic'" />
      <UButton size="sm" color="neutral" :variant="metric === 'cost' ? 'soft' : 'ghost'" :aria-pressed="metric === 'cost'" :disabled="!history.costAvailable" :label="t('paygHistory.costShare')" @click="metric = 'cost'" />
    </div>
    <p v-if="!history.costAvailable" class="text-xs text-muted">{{ t('paygHistory.costUnavailable') }}</p>
    <p v-if="!segments.length" role="status" class="py-6 text-sm text-muted">{{ t(metric === 'cost' ? 'paygHistory.costEmpty' : 'paygHistory.empty') }}</p>
    <template v-else>
      <div class="payg-usage-pie__body">
        <ChartContainer role="img" :config="{}" :aria-label="t('paygHistory.title')">
          <VisSingleContainer :data="segments" :height="220" :duration="0"><VisDonut :value="(d: typeof segments[number]) => d.share" :color="(d: typeof segments[number]) => d.color" :arc-width="0" :events="events" /></VisSingleContainer>
        </ChartContainer>
        <ul class="grid gap-1 min-w-0 content-start">
          <li v-for="segment in segments" :key="segment.nodeId">
            <UButton type="button" color="neutral" variant="ghost" class="payg-usage-pie__legend" :class="{ 'payg-usage-pie__legend--active': activeItem?.nodeId === segment.nodeId }" :aria-pressed="isSelected(segment.nodeId)" @pointerenter="activate(segment.nodeId)" @pointerleave="deactivate(segment.nodeId)" @focus="activate(segment.nodeId)" @blur="deactivate(segment.nodeId)" @click="select(segment.nodeId)">
              <span class="h-2 w-2 rounded-sm shrink-0" :style="{ background: segment.color }" aria-hidden="true" />
              <span class="min-w-0 break-words">{{ segment.name }}</span>
              <span class="ml-auto whitespace-nowrap text-muted">{{ formatStatisticPercent(segment.percentage) }}</span>
            </UButton>
          </li>
        </ul>
      </div>
      <output v-if="activeItem" class="sr-only" aria-live="polite">{{ activeItem.name }}: {{ formatStatisticPercent(activeItem.percentage) }}</output>
    </template>
  </div>
</template>

<style scoped>
.payg-usage-pie { display: grid; gap: .4rem; min-width: 0; }
.payg-usage-pie__body { display: grid; grid-template-columns: minmax(0,1fr); gap: 1rem; }
.payg-usage-pie__legend { display: flex; align-items: center; width: 100%; text-align: left; gap: .5rem; padding: .5rem; min-height: 44px; font-size: .8rem; border-radius: .35rem; }
.payg-usage-pie__legend--active, .payg-usage-pie__legend[aria-pressed="true"] { background: var(--surface); }
.payg-usage-pie__legend:focus-visible { outline: 2px solid var(--accent); outline-offset: 2px; }
@media(min-width:600px) { .payg-usage-pie__body { grid-template-columns: minmax(0,1fr) minmax(0,1fr); align-items: center; } }
@media(prefers-reduced-motion:no-preference) { .payg-usage-pie__legend { transition: background-color .12s; } .payg-usage-pie__legend:active { transform: scale(.99); } }
</style>
