<script setup lang="ts">
import { computed } from 'vue'
import { VisDonut, VisSingleContainer } from '@unovis/vue'
import { ChartContainer } from '@/components/charts'
import { useI18n } from '@/i18n'
import { formatBytes } from '@/utils/format'

const props = defineProps<{ allowanceBytes: string; usedBytes: string }>()
const { t } = useI18n()

function safe(value: string | undefined): bigint {
  try {
    return value && /^\d+$/.test(value) ? BigInt(value) : 0n
  } catch {
    return 0n
  }
}

const allowance = computed(() => safe(props.allowanceBytes))
const used = computed(() => {
  const value = safe(props.usedBytes)
  return value > allowance.value ? allowance.value : value
})
const remaining = computed(() => allowance.value - used.value)
const percentage = computed(() => allowance.value > 0n ? Number(used.value * 1000n / allowance.value) / 10 : 0)
const hasData = computed(() => allowance.value > 0n)
const segments = computed(() => [
  { id: 'used', label: t('dashboard.usedTraffic'), bytes: used.value, value: Number(used.value), color: '#a6d9bb' },
  { id: 'remaining', label: t('dashboard.remainingTraffic'), bytes: remaining.value, value: Number(remaining.value), color: '#3c4a43' },
].filter(segment => segment.bytes > 0n))
</script>

<template>
  <figure class="hourly-allowance" data-hourly-allowance>
    <figcaption>
      <span>{{ t('hourly.dailyAllowance') }}</span>
      <strong>{{ formatBytes(allowance.toString()) }}</strong>
    </figcaption>
    <div v-if="hasData" class="hourly-allowance__body">
      <ChartContainer role="img" :config="{}" :aria-label="t('hourly.allowanceChart')" class="hourly-allowance__chart">
        <VisSingleContainer :data="segments" :height="150" :duration="0">
          <VisDonut :value="(d: typeof segments[number]) => d.value" :color="(d: typeof segments[number]) => d.color" :arc-width="18" :central-label="`${percentage}%`" :pad-angle=".02" />
        </VisSingleContainer>
      </ChartContainer>
      <ul class="hourly-allowance__legend">
        <li v-for="segment in segments" :key="segment.id">
          <span class="hourly-allowance__swatch" :style="{ background: segment.color }" aria-hidden="true" />
          <span>{{ segment.label }}</span>
          <strong>{{ formatBytes(segment.bytes.toString()) }}</strong>
        </li>
      </ul>
    </div>
    <p v-else class="hourly-allowance__empty">{{ t('hourly.allowanceNone') }}</p>
  </figure>
</template>

<style scoped>
.hourly-allowance { display: grid; gap: .4rem; margin: 0; min-width: 0; }
.hourly-allowance figcaption { display: flex; align-items: baseline; justify-content: space-between; gap: .75rem; color: var(--text-muted); font-size: .75rem; }
.hourly-allowance figcaption strong { color: var(--text); font-family: var(--font-mono); font-size: .82rem; }
.hourly-allowance__body { display: grid; grid-template-columns: minmax(0, 9rem) minmax(0, 1fr); align-items: center; gap: .75rem; }
.hourly-allowance__chart { height: 150px; }
.hourly-allowance__legend { display: grid; gap: .3rem; margin: 0; padding: 0; list-style: none; min-width: 0; }
.hourly-allowance__legend li { display: flex; align-items: baseline; gap: .4rem; min-width: 0; font-size: .72rem; color: var(--text-muted); }
.hourly-allowance__legend li strong { margin-inline-start: auto; color: var(--text); font-family: var(--font-mono); font-weight: 600; white-space: nowrap; }
.hourly-allowance__swatch { width: .55rem; height: .55rem; flex: 0 0 auto; border-radius: 2px; }
.hourly-allowance__legend li > span:nth-child(2) { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.hourly-allowance__empty { margin: 0; color: var(--text-muted); font-size: .78rem; }
@media (max-width: 420px) { .hourly-allowance__body { grid-template-columns: minmax(0, 1fr); justify-items: center; } .hourly-allowance__legend { width: 100%; } }
</style>
