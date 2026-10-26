// Adapted from shadcn-vue's MIT-licensed chart composition.
import type { Ref, InjectionKey } from 'vue'
import { inject, provide } from 'vue'
export type ChartConfig = Record<string, { label: string; color: string; dashed?: boolean }>
export type ChartDatum = { x: number; label: string; [key: string]: string | number | undefined }
export const chartContext: InjectionKey<Ref<ChartConfig>> = Symbol('Chart')
export const provideChartContext = (config: Ref<ChartConfig>): void => provide(chartContext, config)
export function useChart(): Ref<ChartConfig> {
  const config = inject(chartContext)
  if (!config) throw new Error('CHART_CONTEXT_REQUIRED')
  return config
}
export { default as ChartContainer } from './ChartContainer.vue'
export { default as ChartTooltipContent } from './ChartTooltipContent.vue'
export { default as ChartLegendContent } from './ChartLegendContent.vue'
export { VisCrosshair as ChartCrosshair, VisTooltip as ChartTooltip } from '@unovis/vue'
export { componentToString } from './tooltip'
