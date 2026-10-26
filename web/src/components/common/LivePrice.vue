<script setup lang="ts">
import type { components } from '@/api/generated'
import type { DeepReadonly } from '@/api/types'
import { useI18n } from '@/i18n'
defineProps<{ decision?: DeepReadonly<components['schemas']['PricingDecision']> | null }>()
const { t } = useI18n()
</script>
<template><small v-if="decision?.dynamicEnabled" class="live-price">{{ t('pricing.livePrice') }}</small></template>
<style scoped>.live-price { display: inline-flex; align-items: center; gap: .3rem; color: var(--text-muted); font-size: .65rem; font-weight: 500; }.live-price::before { content: ''; width: .3rem; height: .3rem; border-radius: 50%; background: var(--accent); }</style>
