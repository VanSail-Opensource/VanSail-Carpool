<script setup lang="ts">
import { computed, shallowRef, watch } from 'vue'
import type { components } from '@/api/generated'
import type { DeepReadonly } from '@/api/types'
import { useI18n } from '@/i18n'
const props = defineProps<{ decision?: DeepReadonly<components['schemas']['PricingDecision']> | null }>()
const { t } = useI18n()
const expired = shallowRef(false)
watch(() => props.decision?.validUntil, (value, _, cleanup) => {
  const remaining = value ? Date.parse(value) - Date.now() : 0
  expired.value = remaining <= 0
  if (remaining > 0) { const timer = globalThis.setTimeout(() => { expired.value = true }, Math.min(remaining, 2_147_483_647)); cleanup(() => globalThis.clearTimeout(timer)) }
}, { immediate: true })
const state = computed(() => props.decision?.status === 'PENDING' ? 'calculating' : props.decision?.status === 'FAILED' ? 'unavailable' : expired.value || props.decision?.status === 'STALE' ? 'stale' : 'ready')
</script>
<template><small v-if="decision?.dynamicEnabled" class="live-price" :data-state="state">{{ t(`pricing.priceStates.${state}`) }}</small></template>
<style scoped>.live-price { display: inline-flex; align-items: center; gap: .3rem; color: var(--text-muted); font-size: .65rem; font-weight: 500; }.live-price::before { content: ''; width: .3rem; height: .3rem; border-radius: 50%; background: var(--accent); }</style>
