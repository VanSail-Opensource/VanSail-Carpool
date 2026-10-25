<script setup lang="ts">
import { useI18n } from '@/i18n'
const { t } = useI18n()
</script>

<template>
  <section class="catalog-blocked" role="status">
    <UIcon name="i-ph-prohibit-bold" class="catalog-blocked__symbol" aria-hidden="true" />
    <h2>{{ t('payg.comboBlocked') }}</h2>
    <p>{{ t('home.autoRenewalCatalogBlocked') }}</p>
    <UButton to="/settings" color="neutral" variant="outline" :label="t('settings.title')" />
  </section>
</template>

<style scoped>
.catalog-blocked { display: grid; justify-items: center; gap: .75rem; padding: 2rem 1rem; text-align: center; }
.catalog-blocked__symbol { width: 3rem; height: 3rem; color: var(--ui-error); animation: prohibition 1.8s ease-in-out infinite; }
.catalog-blocked h2 { margin: 0; font-size: 1rem; }
.catalog-blocked p { margin: 0; max-width: 28rem; font-size: .85rem; color: var(--text-muted); }
@keyframes prohibition { 0%, 100% { opacity: .6; transform: scale(.94); } 50% { opacity: 1; transform: scale(1); } }
@media (prefers-reduced-motion: reduce) { .catalog-blocked__symbol { animation: none; } }
</style>
