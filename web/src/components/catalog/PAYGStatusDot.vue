<script setup lang="ts">
import { computed } from 'vue'
import type { PAYGSquad } from '@/api/payg'
import { useI18n } from '@/i18n'
const props = defineProps<{ status: PAYGSquad['status'] }>()
const { t } = useI18n()
const tone = computed(() => props.status === 'available' ? 'green'
  : props.status === 'insufficient_balance' ? 'red'
    : ['disabled', 'combo_covered', 'queued_combo'].includes(props.status) ? 'grey' : 'yellow')
</script>

<template>
  <UPopover>
    <button type="button" class="payg-status-trigger" :aria-label="t(`payg.status.${status}`)"><span class="payg-status-dot" :class="`payg-status-dot--${tone}`" aria-hidden="true" /></button>
    <template #content><p class="px-3 py-2 text-sm">{{ t(`payg.status.${status}`) }}</p></template>
  </UPopover>
</template>

<style scoped>
.payg-status-trigger { display: inline-grid; place-items: center; width: 1.5rem; height: 1.5rem; border: 0; padding: 0; border-radius: 50%; background: transparent; cursor: pointer; flex: none; }
.payg-status-trigger:focus-visible { outline: 2px solid var(--accent); outline-offset: 2px; }
.payg-status-trigger:active { background: var(--surface-raised); }
.payg-status-dot { position: relative; display: block; width: .4rem; height: .4rem; border-radius: 50%; }
.payg-status-dot::before, .payg-status-dot::after { content: ''; position: absolute; inset: 0; border-radius: inherit; }
.payg-status-dot::before { background: var(--dot-color); animation: payg-status 2.2s ease-in-out infinite; }
.payg-status-dot::after { border: 1px solid var(--dot-color); animation: payg-ripple 2.2s ease-out infinite; }
.payg-status-dot--green { --dot-color: var(--ui-success); }
.payg-status-dot--grey { --dot-color: var(--ui-text-dimmed); }
.payg-status-dot--red { --dot-color: var(--ui-error); }
.payg-status-dot--yellow { --dot-color: var(--ui-warning); }
.payg-status-dot:focus-visible { outline: 2px solid var(--dot-color); outline-offset: 4px; }
@keyframes payg-status { 0%, 100% { opacity: .35; } 45%, 60% { opacity: 1; } }
@keyframes payg-ripple { 0% { opacity: .65; transform: scale(1); } 80%, 100% { opacity: 0; transform: scale(3.2); } }
@media (prefers-reduced-motion: reduce) { .payg-status-dot::before, .payg-status-dot::after { animation: none; } .payg-status-dot::after { display: none; } }
</style>
