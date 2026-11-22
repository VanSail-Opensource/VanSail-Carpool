<script setup lang="ts">
import { onScopeDispose, useTemplateRef, watch } from 'vue'
import { useMotionPreferences } from '@/composables/useMotionPreferences'
import { useI18n } from '@/i18n'

const props = defineProps<{ value: string | null; unit?: string }>()
const { t } = useI18n()
const { reducedMotion } = useMotionPreferences()
const number = useTemplateRef<InstanceType<typeof globalThis.HTMLSpanElement>>('number')
let animation: ReturnType<NonNullable<typeof number.value>['animate']> | undefined

// Keep the current text in one DOM node: no counted-up estimates, duplicate
// screen-reader values, remounts, timers, or animation on the first observation.
watch([() => props.value, reducedMotion], ([value, reduced], [previous]) => {
  animation?.cancel()
  animation = undefined
  if (reduced || value == null || previous == null || value === previous || globalThis.document.hidden) return
  animation = number.value?.animate?.([
    { opacity: 0.55, transform: 'translateY(2px)' },
    { opacity: 1, transform: 'translateY(0)' },
  ], { duration: 200, easing: 'ease-out' })
}, { flush: 'post' })

onScopeDispose(() => animation?.cancel())
</script>

<template>
  <span class="member-metric-value" :class="{ 'member-metric-value--unknown': value == null }">
    <span ref="number" class="member-metric-value__number">{{ value ?? t('common.notAvailable') }}</span>
    <span v-if="value != null && unit" class="member-metric-value__unit">{{ unit }}</span>
  </span>
</template>

<style scoped>
.member-metric-value { display: inline-flex; flex-wrap: wrap; align-items: baseline; gap: .15rem .35rem; max-width: 100%; font-variant-numeric: tabular-nums; }
.member-metric-value__number { display: inline-block; min-width: 0; overflow-wrap: anywhere; }
.member-metric-value__unit { color: var(--text-muted); font-size: .72rem; font-weight: 400; }
.member-metric-value--unknown { color: var(--text-muted); font-size: .8rem; font-weight: 400; }
</style>
