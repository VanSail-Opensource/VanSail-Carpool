<script setup lang="ts">
import { useI18n } from '@/i18n'

defineProps<{ mode: 'payg' | 'combo' }>()
const { t } = useI18n()
</script>

<template>
  <div :key="mode" class="mode-intro" :class="`mode-intro--${mode}`">
    <div class="mode-intro__line">
      <p aria-live="polite">{{ t(mode === 'payg' ? 'payg.automatic' : 'payg.comboIntro') }}</p>
      <details v-if="mode === 'payg'" class="mode-intro__details">
        <summary><UIcon name="i-ph-info" aria-hidden="true" /><span class="sr-only">{{ t('payg.billingDetails') }}</span></summary>
        <p>{{ t('payg.billingDetailsText') }}</p>
      </details>
    </div>
  </div>
</template>

<style scoped>
.mode-intro { min-width: 0; padding-block: .1rem .3rem; }
.mode-intro__line { display: flex; flex-wrap: wrap; align-items: center; gap: .35rem .7rem; }
.mode-intro__line > p { flex: 1; min-width: 8rem; margin: 0; color: var(--text-muted); font-size: .75rem; line-height: 1.5; }
.mode-intro__details { display: contents; }
.mode-intro__details summary { display: grid; place-items: center; width: 44px; min-height: 44px; margin-left: auto; color: var(--text-muted); cursor: pointer; list-style: none; border-radius: var(--radius-control); }
.mode-intro__details summary::-webkit-details-marker { display: none; }
.mode-intro__details summary:hover, .mode-intro__details[open] summary { color: var(--text); background: var(--surface-raised); }
.mode-intro__details summary:focus-visible { outline: 2px solid var(--accent); outline-offset: 2px; }
.mode-intro__details > p { flex: 1 0 100%; margin: 0; padding: .5rem .65rem; border-left: 2px solid var(--line); color: var(--text-muted); font-size: .75rem; line-height: 1.6; }
</style>
