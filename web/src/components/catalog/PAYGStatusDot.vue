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
  <UTooltip :text="t(`payg.status.${status}`)">
    <span class="payg-status-dot" :class="`payg-status-dot--${tone}`" role="img" :aria-label="t(`payg.status.${status}`)" tabindex="0" />
  </UTooltip>
</template>

<style scoped>
.payg-status-dot { position: absolute; left: .6rem; top: 1rem; width: .4rem; height: .4rem; border-radius: 50%; }
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
