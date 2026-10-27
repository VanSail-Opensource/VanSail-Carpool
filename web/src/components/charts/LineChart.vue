<script setup lang="ts">
import { computed, shallowRef, watch } from 'vue'
import { VisXYContainer, VisLine, VisArea, VisAxis, VisGroupedBar, VisStackedBar } from '@unovis/vue'
import { CurveType } from '@unovis/ts'
import { ChartContainer, ChartCrosshair, ChartTooltip, ChartTooltipContent, ChartLegendContent, componentToString } from './index'
import type { ChartConfig, ChartDatum } from './index'
import { useI18n } from '@/i18n'
import { bridgeSeries } from './bridge'
const props = withDefaults(defineProps<{ data: readonly ChartDatum[]; config: ChartConfig; label: string; formatValue?: (value: number) => string; formatX?: (value: number) => string; band?: { low: string; high: string; color: string }; height?: number; bar?: boolean; stacked?: boolean; legend?: boolean; connectMissing?: boolean; bridge?: { from: string; to: string; color: string } }>(), { height: 200, legend: true, formatValue: undefined, formatX: undefined, band: undefined, bridge: undefined })
const { t } = useI18n()
const selected = shallowRef<number | undefined>()
const forceX = shallowRef<number | undefined>()
const rows = computed(() => [...props.data])
const bridgeRows = computed(() => props.bridge ? bridgeSeries(rows.value, props.bridge.from, props.bridge.to) : [])
const bandSegments = computed(() => {
 const groups: ChartDatum[][] = []
 let current: ChartDatum[] = []
 for (const row of rows.value) {
  if (props.band && Number.isFinite(row[props.band.low]) && Number.isFinite(row[props.band.high])) { current.push(row) }
  else if (current.length) { groups.push(current); current = [] }
 }
 if (current.length) groups.push(current)
 return groups
})
const accessors = computed(() => Object.keys(props.config).map(key => (d: ChartDatum) => Number(d[key] ?? NaN)))
const point = computed(() => selected.value === undefined ? undefined : rows.value[selected.value])
const tooltipConfig = computed(() => ({ ...props.config, ...(props.band ? { [props.band.low]: { label: t('charts.lower'), color: props.band.color }, [props.band.high]: { label: t('charts.upper'), color: props.band.color } } : {}) }))
const tooltip = computed(() => componentToString(tooltipConfig.value, ChartTooltipContent, { labelFormatter: (x: number | Date) => rows.value.find(row => row.x === Number(x))?.label || props.formatX?.(Number(x)) || String(x), valueFormatter: props.formatValue, missingLabel: props.connectMissing ? t('common.notAvailable') : undefined }))
function inspect(index: number): void {
  if (!rows.value.length) return
  selected.value = Math.max(0, Math.min(rows.value.length - 1, index))
  forceX.value = rows.value[selected.value]?.x
}
function keydown(event: globalThis.KeyboardEvent): void {
  const current = selected.value ?? 0
  const next = event.key === 'ArrowLeft' ? current - 1 : event.key === 'ArrowRight' ? current + 1 : event.key === 'Home' ? 0 : event.key === 'End' ? rows.value.length - 1 : undefined
  if (next !== undefined) { event.preventDefault(); inspect(next) }
}
function touch(event: globalThis.PointerEvent): void {
  if (event.pointerType !== 'touch') { forceX.value = undefined; return }
  const rect = (event.currentTarget as globalThis.HTMLElement).getBoundingClientRect()
  const ratio = Math.max(0, Math.min(1, (event.clientX - rect.left - 36) / Math.max(1, rect.width - 48)))
  const start = rows.value[0]?.x ?? 0
  const at = start + ratio * ((rows.value.at(-1)?.x ?? start) - start)
  inspect(rows.value.reduce((best, row, index) => Math.abs(row.x - at) < Math.abs(rows.value[best]!.x - at) ? index : best, 0))
}
function moved(_x?: number | Date, _datum?: ChartDatum, index?: number): void { if (index !== undefined) selected.value = index }
const x = (d: ChartDatum) => d.x
watch(() => props.data, () => { selected.value = undefined; forceX.value = undefined })
</script>
<template>
  <ChartContainer :config="config">
    <div class="line-chart__plot" tabindex="0" role="group" :aria-label="label" @keydown="keydown" @pointerdown="touch" @pointermove="touch">
      <VisXYContainer :height="height" :duration="0" :y-domain="[0, undefined]" :margin="{ left: 8, right: 8, top: 8, bottom: 0 }">
        <VisArea v-for="(segment, index) in bandSegments" :key="index" :data="segment" :x="x" :y="(d: ChartDatum) => Number(d[band!.high]) - Number(d[band!.low])" :baseline="(d: ChartDatum) => Number(d[band!.low] ?? NaN)" :color="band!.color" :opacity=".12" :curve-type="CurveType.Linear" />
        <component :is="stacked ? VisStackedBar : VisGroupedBar" v-if="bar" :data="rows" :x="x" :y="accessors" :color="Object.values(config).map(item => item.color)" />
        <template v-if="!bar"><VisLine v-for="(item, key, index) in config" :key="key" :data="rows" :x="x" :y="accessors[index]!" :color="item.color" :line-dash-array="item.dashed ? [5, 4] : undefined" :curve-type="CurveType.Linear" :fallback-value="undefined" :interpolate-missing-data="connectMissing" /><VisLine v-if="bridgeRows.length" :data="bridgeRows" :x="x" :y="(d: ChartDatum) => Number(d.bridgeValue)" :color="bridge!.color" :line-dash-array="[5, 4]" :curve-type="CurveType.Linear" /></template>
        <VisAxis type="x" :num-ticks="Math.min(5, rows.length)" :tick-format="(n: number) => formatX?.(n) || rows.find(row => row.x === n)?.label || ''" :grid-line="false" />
        <VisAxis type="y" :num-ticks="3" :tick-format="formatValue" :domain-line="false" :tick-line="false" />
        <ChartTooltip />
        <ChartCrosshair :data="rows" :x="x" :y="accessors" :color="Object.values(config).map(item => item.color)" :template="tooltip" :force-show-at="forceX" :hide-when-far-from-pointer="false" :on-crosshair-move="moved" />
      </VisXYContainer>
    </div>
    <ChartLegendContent v-if="legend" />
    <div v-if="rows.length" class="line-chart__inspection">
      <UButton icon="i-ph-caret-left" size="xs" color="neutral" variant="ghost" :aria-label="t('charts.previous')" @click="inspect((selected ?? 1) - 1)" />
      <output aria-live="polite"><template v-if="point"><strong>{{ point.label }}</strong><span v-for="(item, key) in tooltipConfig" :key="key">{{ item.label }}: {{ typeof point[key] === 'number' && Number.isFinite(point[key]) ? (formatValue?.(Number(point[key])) ?? point[key]) : t('common.notAvailable') }}</span></template><span v-else>{{ t('charts.inspect') }}</span></output>
      <UButton icon="i-ph-caret-right" size="xs" color="neutral" variant="ghost" :aria-label="t('charts.next')" @click="inspect((selected ?? -1) + 1)" />
    </div>
  </ChartContainer>
</template>
<style scoped>
.line-chart__plot { min-width: 0; touch-action: pan-y; }.line-chart__plot:focus-visible { outline: 2px solid var(--accent); outline-offset: 2px; border-radius: .25rem; }
.line-chart__inspection { display: flex; gap: .5rem; align-items: center; justify-content: space-between; min-height: 2.5rem; }.line-chart__inspection output { display: flex; flex-wrap: wrap; justify-content: center; gap: .25rem .65rem; color: var(--text-muted); font-size: .7rem; }
</style>
