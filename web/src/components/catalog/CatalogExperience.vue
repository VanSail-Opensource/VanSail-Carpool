<script setup lang="ts">
import { shallowRef } from 'vue'
import { useI18n } from '@/i18n'
import CatalogPage from './CatalogPage.vue'
import PAYGPage from './PAYGPage.vue'

const { t } = useI18n()
const mode = shallowRef<'payg' | 'combo'>('payg')
</script>

<template>
  <div class="page catalog-experience">
    <header class="catalog-experience__header">
      <h1>{{ t(mode === 'payg' ? 'payg.title' : 'catalog.title') }}</h1>
      <div class="catalog-experience__modes" role="group" :aria-label="t('payg.mode')">
        <UButton color="neutral" :variant="mode === 'payg' ? 'solid' : 'ghost'" :aria-pressed="mode === 'payg'" :label="t('payg.title')" @click="mode = 'payg'" />
        <UButton color="neutral" :variant="mode === 'combo' ? 'solid' : 'ghost'" :aria-pressed="mode === 'combo'" :label="t('payg.combo')" @click="mode = 'combo'" />
      </div>
    </header>
    <Transition name="catalog-mode" mode="out-in">
      <PAYGPage v-if="mode === 'payg'" />
      <CatalogPage v-else embedded />
    </Transition>
  </div>
</template>

<style scoped>
.catalog-experience__header { display: flex; justify-content: space-between; align-items: center; gap: 0.75rem; margin-bottom: 1rem; }
.catalog-experience__header h1 { margin: 0; font-size: 1.15rem; font-weight: 650; }
.catalog-experience__modes { display: flex; flex-shrink: 0; gap: 0.15rem; }
.catalog-mode-enter-active, .catalog-mode-leave-active { transition: opacity 160ms ease, transform 160ms ease; }
.catalog-mode-enter-from { opacity: 0; transform: translateY(8px); }
.catalog-mode-leave-to { opacity: 0; transform: translateY(-4px); }
@media (max-width: 430px) { .catalog-experience__header { align-items: flex-start; flex-direction: column; } .catalog-experience__modes { align-self: flex-end; } }
@media (prefers-reduced-motion: reduce) { .catalog-mode-enter-active, .catalog-mode-leave-active { transition: opacity 80ms ease; } .catalog-mode-enter-from, .catalog-mode-leave-to { transform: none; } }
</style>
