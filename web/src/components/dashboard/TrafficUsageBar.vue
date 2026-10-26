<script setup lang="ts">
import { computed, shallowRef } from 'vue'
import { VisStackedBar, VisXYContainer } from '@unovis/vue'
import { ChartContainer } from '@/components/charts'

import type { CatalogNode, TopNode } from '@/api/types'
import { getLocale, t } from '@/i18n'
import { formatBytes } from '@/utils/format'
import { selectionHaptic } from '@/utils/telegram'
import TrafficUsageBarDetails from './TrafficUsageBarDetails.vue'
const props = defineProps<{
  nodes: readonly TopNode[]
  totalBytes: string
  catalogNodes: readonly CatalogNode[]
  useMultiplier: boolean
}>()

const scale = 10_000n
const pointerScale = 1_000_000n
const multiplierScale = 1_000n
const paletteSize = 5
const minimumSharePercent = 3n
const hoveredKey = shallowRef<string | null>(null)
const selectedKey = shallowRef<string | null>(null)
const trackRef = shallowRef<globalThis.HTMLElement | null>(null)

export interface TrafficUsageSegment { key: string; name: string; countryCode: string; bytes: bigint; startBytes: bigint; widthBasis: bigint; color: string; bytesLabel: string; percentageLabel: string; multiplierLabel: string; isOther: boolean; ariaLabel: string }
interface SourceNode { node: TopNode; index: number; bytes: bigint; calculatedBytes: bigint; multiplier: number | null }

function byteCount(value: string): bigint {
  try {
    return /^\d+$/.test(value.trim()) ? BigInt(value.trim()) : 0n
  } catch {
    return 0n
  }
}

function percentageLabel(value: bigint, total: bigint): string {
  const tenths = total > 0n ? (value * 1000n) / total : 0n
  return new Intl.NumberFormat(getLocale(), { minimumFractionDigits: 1, maximumFractionDigits: 1 }).format(Number(tenths) / 10)
}

function multiplierLabel(value: number | null): string {
  if (value === null || !Number.isFinite(value)) return t('home.trafficMultiplierUnavailable')
  const formatted = new Intl.NumberFormat(getLocale(), { maximumFractionDigits: 2 }).format(value)
  return t('home.trafficMultiplierValue', { value: formatted })
}

function finiteMultiplier(value: unknown): number | null { return typeof value === 'number' && Number.isFinite(value) ? value : null }
const catalogByUuid = computed(() => new Map(props.catalogNodes.map((node) => [node.uuid, node])))
function multiplierFactor(value: number | null): bigint {
  if (value === null || !Number.isFinite(value) || value < 0) return multiplierScale
  return BigInt(Math.max(0, Math.round(value * Number(multiplierScale))))
}

const sourceNodes = computed<SourceNode[]>(() => props.nodes
  .map((node, index) => {
    const bytes = byteCount(node.totalBytes)
    const metadata = catalogByUuid.value.get(node.uuid)
    const multiplier = finiteMultiplier(node.consumptionMultiplier) ?? finiteMultiplier(metadata?.consumptionMultiplier)
    const calculatedBytes = props.useMultiplier
      ? (bytes * multiplierFactor(multiplier)) / multiplierScale
      : bytes
    return { node, index, bytes, calculatedBytes, multiplier }
  })
  .filter((entry) => entry.bytes > 0n))

const totalForBar = computed(() => {
  const reported = byteCount(props.totalBytes)
  const listed = sourceNodes.value.reduce((sum, entry) => sum + entry.calculatedBytes, 0n)
  if (!props.useMultiplier) {
    return listed > 0n ? listed : reported
  }
  return reported > listed ? reported : listed
})

const colorByUuid = computed(() => {
  const uuids = [...new Set(sourceNodes.value.map(({ node }) => node.uuid))].sort()
  return new Map(uuids.map((uuid, index) => [uuid, index % paletteSize]))
})

const segments = computed<TrafficUsageSegment[]>(() => {
  const total = totalForBar.value
  if (total <= 0n || sourceNodes.value.length === 0) return []
  const visibleNodes = sourceNodes.value.filter((entry) => entry.calculatedBytes * 100n >= total * minimumSharePercent)
  const visibleBytes = visibleNodes.reduce((sum, entry) => sum + entry.calculatedBytes, 0n)
  const entries = visibleNodes.map(({ node, index, calculatedBytes, multiplier }) => {
    const metadata = catalogByUuid.value.get(node.uuid)
    return {
      key: node.uuid + '-' + index,
      name: metadata?.name || node.name,
      countryCode: metadata?.countryCode || node.countryCode,
      bytes: calculatedBytes,
      rawWidthBasis: (calculatedBytes * scale) / total,
      color: 'var(--traffic-node-' + ((colorByUuid.value.get(node.uuid) ?? index % paletteSize) + 1) + ')',
      multiplierLabel: multiplierLabel(multiplier),
      isOther: false,
    }
  })
  if (total > visibleBytes) {
    entries.push({
      key: 'other-usage',
      name: t('home.trafficOther'),
      countryCode: '',
      bytes: total - visibleBytes,
      rawWidthBasis: ((total - visibleBytes) * scale) / total,
      color: 'var(--traffic-node-other)',
      multiplierLabel: t('home.trafficMultiplierUnavailable'),
      isOther: true,
    })
  }
  let startBytes = 0n
  let widthBasisUsed = 0n
  return entries.map((entry, index) => {
    const widthBasis = index === entries.length - 1 ? scale - widthBasisUsed : entry.rawWidthBasis
    widthBasisUsed += widthBasis
    const bytesLabel = formatBytes(entry.bytes.toString())
    const percentage = percentageLabel(entry.bytes, total)
    const ariaLabel = entry.isOther
      ? t('home.trafficOtherAria', { usage: bytesLabel, percentage })
      : t('home.trafficSegmentAria', { name: entry.name, country: entry.countryCode, usage: bytesLabel, percentage, multiplier: entry.multiplierLabel })
    const segment: TrafficUsageSegment = { ...entry, startBytes, widthBasis, bytesLabel, percentageLabel: percentage, ariaLabel }
    startBytes += entry.bytes
    return segment
  })
})

const activeSegment = computed(() => {
  const key = hoveredKey.value ?? selectedKey.value
  return segments.value.find((segment) => segment.key === key) ?? null
})

function segmentAt(clientX: number): TrafficUsageSegment | null {
  const rect = trackRef.value?.getBoundingClientRect()
  const total = totalForBar.value
  if (!rect || rect.width <= 0 || total <= 0n) return null
  const relative = Math.min(0.999999, Math.max(0, (clientX - rect.left) / rect.width))
  const position = BigInt(Math.floor(relative * Number(pointerScale)))
  return segments.value.find((segment) => position * total < (segment.startBytes + segment.bytes) * pointerScale) ?? null
}

function previewAt(event: globalThis.PointerEvent): void { hoveredKey.value = segmentAt(event.clientX)?.key ?? null }
function selectAt(event: globalThis.MouseEvent): void { const segment = segmentAt(event.clientX); if (segment) toggleSelection(segment.key) }
function toggleSelection(key: string): void {
  selectionHaptic()
  if (selectedKey.value === key) { selectedKey.value = null; hoveredKey.value = null; return }
  selectedKey.value = key
}
function clearHover(): void { hoveredKey.value = null }
function clearSelection(): void { selectedKey.value = null; hoveredKey.value = null }
</script>

<template>
  <div v-if="segments.length" class="traffic-usage-bar">
    <div ref="trackRef" class="traffic-usage-bar__track" role="group" :aria-label="$t('home.trafficBarLabel')" @click="selectAt" @pointermove="previewAt" @pointerleave="clearHover" @pointercancel="clearHover" @keydown.escape.stop.prevent="clearSelection">
      <ChartContainer :config="{}" class="traffic-usage-bar__unovis"><VisXYContainer :data="[{ x: 0 }]" :height="44" :duration="0" :margin="{ top: 0, bottom: 0, left: 0, right: 0 }" :x-domain="[0, 100]" :y-domain="[-.5, .5]"><VisStackedBar :x="() => 0" :y="segments.map(segment => () => Number(segment.widthBasis) / 100)" :color="segments.map(segment => segment.color)" orientation="horizontal" :bar-width="44" :rounded-corners="false" /></VisXYContainer></ChartContainer>
      <UButton v-for="(segment, index) in segments" :key="segment.key" type="button" color="neutral" variant="ghost" class="traffic-usage-bar__segment" :class="{ 'traffic-usage-bar__segment--selected': selectedKey === segment.key, 'traffic-usage-bar__segment--first': index === 0, 'traffic-usage-bar__segment--last': index === segments.length - 1 }" :style="{ left: Number(segment.startBytes * scale / totalForBar) / 100 + '%', width: Number(segment.widthBasis) / 100 + '%' }" :aria-label="segment.ariaLabel" :aria-pressed="selectedKey === segment.key" @click.stop="toggleSelection(segment.key)" @focus="hoveredKey = segment.key" @blur="clearHover" />
    </div>
    <TrafficUsageBarDetails v-if="activeSegment" :key="activeSegment.key" :segment="activeSegment" />
  </div>
</template>
<style scoped>
.traffic-usage-bar { --traffic-node-1: var(--accent); --traffic-node-2: var(--accent-strong); --traffic-node-3: var(--success); --traffic-node-4: var(--warning); --traffic-node-5: var(--text-muted); --traffic-node-other: var(--line-strong); margin-top: 0.75rem; }
.traffic-usage-bar__track { position: relative; display: flex; min-height: 44px; overflow: hidden; border: 1px solid var(--line); border-radius: 12px; background: var(--surface-raised); isolation: isolate; }
.traffic-usage-bar__unovis { position: absolute; inset: 0; pointer-events: none; }
.traffic-usage-bar__visual { display: block; min-width: 0; height: 100%; min-height: 44px; opacity: 0.82; }
.traffic-usage-bar__segment { position: absolute; inset-block: 0; z-index: 1; min-width: 0; padding: 0; border: 0; border-radius: 0; background: transparent; cursor: pointer; }
.traffic-usage-bar__segment--first { border-start-start-radius: inherit; border-end-start-radius: inherit; }
.traffic-usage-bar__segment--last { border-start-end-radius: inherit; border-end-end-radius: inherit; }
.traffic-usage-bar__segment:focus-visible { outline: 2px solid var(--text); outline-offset: -3px; }
.traffic-usage-bar__segment--selected { box-shadow: inset 0 0 0 2px var(--text); }
</style>
