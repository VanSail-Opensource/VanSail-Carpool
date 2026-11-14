<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from '@/i18n'

const props = defineProps<{ step: number; phase: number }>()
const { t } = useI18n()
const ready = computed(() => props.phase >= .48)
const action = computed(() => [ready.value ? 'balanceReady' : 'addBalance', 'billingChoice', 'routingOptional', 'clientDownload', ready.value ? 'refresh' : 'importLink', ready.value ? 'connected' : 'connect'][props.step] ?? 'connect')
const icons = ['i-ph-wallet-bold', 'i-ph-arrows-down-up-bold', 'i-ph-git-branch-bold', 'i-ph-download-simple-bold', 'i-ph-copy-bold', 'i-ph-power-bold']
const clients = ['flclash', 'v2rayn', 'v2rayng'] as const
</script>

<template>
  <div class="subscription-action" :class="{ 'is-ready': ready }" :data-subscription-step="step">
    <template v-if="step === 1"><span><UIcon name="i-ph-clock-bold" />{{ t('knowledge.labels.hourlyPurchase') }}</span><small>{{ t('knowledge.labels.or') }}</small><span><UIcon name="i-ph-arrows-down-up-bold" />{{ t('knowledge.labels.trafficReady') }}</span></template>
    <template v-else-if="step === 3"><span v-for="client in clients" :key="client"><UIcon :name="client === 'v2rayng' ? 'i-ph-device-mobile-bold' : 'i-ph-desktop-bold'" />{{ t(`knowledge.clients.${client}`) }}</span></template>
    <template v-else><UIcon :name="icons[step]" /><span>{{ t(`knowledge.labels.${action}`) }}</span><UIcon :name="ready ? 'i-ph-check-circle-fill' : 'i-ph-arrow-right-bold'" /></template>
  </div>
</template>

<style scoped>
.subscription-action { position: absolute; inset-inline: 12%; bottom: .7rem; display: flex; align-items: center; justify-content: center; gap: .55rem; min-height: 2.25rem; padding: .4rem .6rem; border: 1px solid var(--line); border-radius: .55rem; background: var(--surface-raised); color: var(--text-muted); font-size: .68rem; }
.subscription-action > .iconify { flex: none; color: var(--accent); font-size: 1.05rem; }
.subscription-action > .iconify:last-child { margin-inline-start: auto; }
.subscription-action > span { display: inline-flex; align-items: center; gap: .3rem; }
.subscription-action > small { color: var(--text-faint); }
.subscription-action[data-subscription-step="1"], .subscription-action[data-subscription-step="3"] { flex-wrap: wrap; }
.subscription-action.is-ready { border-color: var(--accent-line); }
@media (max-width: 420px) { .subscription-action { inset-inline: 6%; gap: .45rem; font-size: .6rem; } }
</style>
