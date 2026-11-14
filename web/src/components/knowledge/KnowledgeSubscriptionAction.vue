<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from '@/i18n'
import DemoPointer from '@/components/common/DemoPointer.vue'
import { interactionCue } from '@/components/common/illustrationTimeline'

const props = defineProps<{ step: number; phase: number; reducedMotion: boolean }>()
const { t } = useI18n()
const ready = computed(() => props.phase >= .48)
const action = computed(() => [ready.value ? 'balanceReady' : 'addBalance', 'billingChoice', 'routingOptional', 'clientDownload', ready.value ? 'refresh' : 'importLink', ready.value ? 'connected' : 'connect'][props.step] ?? 'connect')
const icons = ['i-ph-wallet-bold', 'i-ph-arrows-down-up-bold', 'i-ph-git-branch-bold', 'i-ph-download-simple-bold', 'i-ph-copy-bold', 'i-ph-power-bold']
const clients = ['flclash', 'v2rayn', 'v2rayng'] as const
const cue = computed(() => interactionCue(props.phase, props.step === 1
  ? [{ at: .32, x: 30, y: 50 }, { at: .68, x: 75, y: 50 }]
  : props.step === 3 ? [{ at: .28, x: 20, y: 50 }, { at: .56, x: 50, y: 50 }, { at: .82, x: 80, y: 50 }]
    : [{ at: .32, x: 100, y: 50 }, { at: .68, x: 100, y: 50 }]))
</script>

<template>
  <div class="subscription-action" :class="{ 'is-ready': ready }" :data-subscription-step="step">
    <template v-if="step === 1"><span><UIcon name="i-ph-clock-bold" />{{ t('knowledge.labels.hourlyPurchase') }}</span><small>{{ t('knowledge.labels.or') }}</small><span><UIcon name="i-ph-arrows-down-up-bold" />{{ t('knowledge.labels.trafficReady') }}</span></template>
    <template v-else-if="step === 3"><span v-for="client in clients" :key="client"><UIcon :name="client === 'v2rayng' ? 'i-ph-device-mobile-bold' : 'i-ph-desktop-bold'" />{{ t(`knowledge.clients.${client}`) }}</span></template>
    <template v-else><UIcon :name="icons[step]" /><span>{{ t(`knowledge.labels.${action}`) }}</span><UIcon :name="ready ? 'i-ph-check-circle-fill' : 'i-ph-arrow-right-bold'" class="subscription-action__status" /></template>
    <DemoPointer :cue="cue" :reduced-motion="reducedMotion" />
  </div>
</template>

<style scoped>
.subscription-action { position: relative; display: flex; align-items: center; justify-content: center; gap: .55rem; min-width: 0; min-height: 2.6rem; padding: .6rem .9rem; border-top: 1px solid var(--line); background: var(--surface-raised); color: var(--text-muted); font-size: .68rem; --pointer-inset: 1.4rem; }
.subscription-action > .iconify { flex: none; color: var(--accent); font-size: 1.05rem; }
.subscription-action__status { margin-inline-start: auto; }
.subscription-action > span { display: inline-flex; align-items: center; gap: .3rem; }
.subscription-action > small { color: var(--text-faint); }
.subscription-action[data-subscription-step="1"], .subscription-action[data-subscription-step="3"] { flex-wrap: wrap; --pointer-inset: 0px; }
.subscription-action.is-ready { border-top-color: var(--accent-line); }
@media (max-width: 420px) { .subscription-action { gap: .45rem; font-size: .6rem; } }
</style>
