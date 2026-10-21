<script setup lang="ts">
import { computed } from 'vue'
import type { RouteSpeedtestWrite, RoutingView } from '@/api/xnodeMember'
import { useI18n } from '@/i18n'
import { useRouteSpeedtest } from './useRouteSpeedtest'

defineProps<{ eligible: boolean; request: RouteSpeedtestWrite; nodes: RoutingView['nodes']; endpoints: NonNullable<RoutingView['endpoints']> }>()
const { t } = useI18n()
const state = useRouteSpeedtest()
const completed = computed(() => state.result.value?.pairs.filter(pair => pair.status === 'succeeded').length ?? 0)
const median = (values: number[]) => [...values].sort((a, b) => a - b)[Math.floor(values.length / 2)] ?? 0
</script>

<template>
  <div class="grid gap-2 min-w-0">
    <div class="flex flex-wrap items-center gap-2">
      <UButton variant="outline" color="neutral" icon="i-ph-gauge-bold" :label="t('routeSpeedtest.start')" :loading="state.command.busy.value" :disabled="!eligible || state.command.blocksMutations.value" @click="state.start(request)" />
      <span class="text-xs text-muted">{{ t('routeSpeedtest.metered') }}</span>
    </div>
    <p v-if="!eligible && !state.command.receipt.value" class="text-xs text-muted">{{ t('routeSpeedtest.requirements') }}</p>
    <p v-if="state.command.receipt.value" class="text-xs text-muted" role="status">{{ t('routeSpeedtest.progress', { completed, total: state.result.value?.pairs.length ?? 0 }) }}</p>
    <UProgress v-if="state.command.busy.value" />
    <UAlert v-if="state.command.error.value" color="warning" :title="state.command.error.value" />
    <UButton v-if="state.command.error.value && state.command.receipt.value" class="justify-self-start" variant="ghost" :label="t('common.tryAgain')" :loading="state.command.checking.value" @click="state.command.refresh" />
    <div v-for="(pair, index) in state.result.value?.pairs ?? []" :key="index" class="grid gap-1 border-b border-default py-2 text-sm">
      <span class="break-words">{{ nodes.find(node => node.id === pair.sourceNodeId)?.name ?? pair.sourceNodeId }} → {{ endpoints.find(endpoint => endpoint.id === pair.targetEndpointId)?.name ?? pair.targetEndpointId }}</span>
      <span class="text-xs text-muted">{{ t(`routeSpeedtest.status.${pair.status}`) }}</span>
      <span v-if="pair.measurement" class="text-xs">{{ t('routeSpeedtest.measurement', { latency: median(pair.measurement.request_latency_millis).toFixed(1), download: pair.measurement.download_mbps.toFixed(1), upload: pair.measurement.upload_mbps.toFixed(1) }) }}</span>
    </div>
    <p v-if="['pending_review', 'partial'].includes(state.command.receipt.value?.status ?? '')" class="text-sm text-warning">{{ t('routeSpeedtest.uncertain') }}</p>
    <p v-if="state.result.value?.estimate" class="text-sm">{{ t('routeSpeedtest.estimate', { latency: state.result.value.estimate.latencyMillis.toFixed(1), download: state.result.value.estimate.downloadMbps.toFixed(1), upload: state.result.value.estimate.uploadMbps.toFixed(1) }) }}</p>
  </div>
</template>
