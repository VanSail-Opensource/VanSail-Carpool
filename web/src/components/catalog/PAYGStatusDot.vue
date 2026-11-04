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
    <UButton type="button" color="neutral" variant="ghost" class="payg-status-trigger" :aria-label="t(`payg.status.${status}`)"><span class="payg-status-dot" :class="`payg-status-dot--${tone}`" aria-hidden="true" /></UButton>
    <template #content><p class="px-3 py-2 text-sm">{{ t(`payg.status.${status}`) }}</p></template>
  </UPopover>
</template>

<style scoped>
.payg-status-trigger { display: inline-grid; place-items: center; width: 1.5rem; height: 1.5rem; min-width: 1.5rem; min-height: 1.5rem; border: 0; padding: 0; border-radius: 50%; background: transparent; cursor: pointer; flex: none; }
.payg-status-trigger:focus-visible { outline: 2px solid var(--accent); outline-offset: 2px; }
.payg-status-trigger:active { background: var(--surface-raised); }
.payg-status-dot { display: block; width: .4375rem; height: .4375rem; border-radius: 50%; background: var(--dot-color); }
.payg-status-dot--green { --dot-color: var(--ui-success); }
.payg-status-dot--grey { --dot-color: var(--ui-text-dimmed); }
.payg-status-dot--red { --dot-color: var(--ui-error); }
.payg-status-dot--yellow { --dot-color: var(--ui-warning); }
</style>
