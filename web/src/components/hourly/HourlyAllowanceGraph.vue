<script setup lang="ts">
import { computed } from 'vue'
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
const color = computed(() => percentage.value >= 90 ? 'error' : percentage.value >= 70 ? 'warning' : 'primary')
</script>

<template>
  <figure class="hourly-allowance" data-hourly-allowance>
    <figcaption>
      <span>{{ t('hourly.dailyAllowance') }}</span>
      <strong>{{ formatBytes(allowance.toString()) }}</strong>
    </figcaption>
    <div class="hourly-allowance__meter" role="group" :aria-label="t('hourly.allowanceChart')">
      <UProgress :model-value="percentage" :max="100" :color="color" />
      <dl class="hourly-allowance__legend">
        <div><dt><span class="hourly-allowance__swatch hourly-allowance__swatch--used" aria-hidden="true" />{{ t('dashboard.usedTraffic') }}</dt><dd>{{ formatBytes(used.toString()) }}</dd></div>
        <div><dt><span class="hourly-allowance__swatch hourly-allowance__swatch--remaining" aria-hidden="true" />{{ t('dashboard.remainingTraffic') }}</dt><dd>{{ formatBytes(remaining.toString()) }}</dd></div>
      </dl>
    </div>
  </figure>
</template>

<style scoped>
.hourly-allowance { display: grid; gap: .4rem; margin: 0; min-width: 0; }
.hourly-allowance figcaption { display: flex; align-items: baseline; justify-content: space-between; gap: .75rem; color: var(--text-muted); font-size: .75rem; }
.hourly-allowance figcaption strong { color: var(--text); font-family: var(--font-mono); font-size: .82rem; }
.hourly-allowance__meter { display: grid; gap: .45rem; min-width: 0; }
.hourly-allowance__legend { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: .2rem .9rem; margin: 0; }
.hourly-allowance__legend div { display: flex; align-items: baseline; gap: .4rem; min-width: 0; font-size: .72rem; color: var(--text-muted); }
.hourly-allowance__legend dt { display: inline-flex; align-items: center; gap: .3rem; min-width: 0; }
.hourly-allowance__legend dd { margin: 0 0 0 auto; color: var(--text); font-family: var(--font-mono); font-weight: 600; white-space: nowrap; }
.hourly-allowance__swatch { width: .55rem; height: .55rem; flex: 0 0 auto; border-radius: 2px; }
.hourly-allowance__swatch--used { background: var(--accent); }
.hourly-allowance__swatch--remaining { background: var(--line); }
</style>
