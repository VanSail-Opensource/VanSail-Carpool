<script setup lang="ts">
import { shallowRef } from 'vue'
import { useI18n } from '@/i18n'
import CatalogPage from './CatalogPage.vue'
import PAYGPage from './PAYGPage.vue'
import CatalogModeIntro from './CatalogModeIntro.vue'
import CatalogModeTransition from './CatalogModeTransition.vue'

const { t } = useI18n()
const mode = shallowRef<'payg' | 'combo'>('payg')
const transitioning = shallowRef(false)
</script>

<template>
  <div class="page catalog-experience" :class="{ 'catalog-experience--transitioning': transitioning }">
    <CatalogModeTransition :mode="mode" @active="transitioning = $event" />
    <header class="catalog-experience__header">
      <h1 class="sr-only">{{ t(mode === 'payg' ? 'payg.title' : 'catalog.title') }}</h1>
      <div class="catalog-experience__modes" role="group" :aria-label="t('payg.mode')">
        <UButton color="neutral" :variant="mode === 'payg' ? 'solid' : 'ghost'" :aria-pressed="mode === 'payg'" :label="t('payg.title')" @click="mode = 'payg'" />
        <UButton color="neutral" :variant="mode === 'combo' ? 'solid' : 'ghost'" :aria-pressed="mode === 'combo'" :label="t('payg.combo')" @click="mode = 'combo'" />
      </div>
    </header>
    <CatalogModeIntro :mode="mode" />
    <Transition name="catalog-mode" mode="out-in">
      <PAYGPage v-if="mode === 'payg'" />
      <CatalogPage v-else embedded />
    </Transition>
  </div>
</template>

<style scoped>
.catalog-experience__header { margin-bottom: .15rem; }
.catalog-experience__modes { display: flex; gap: .2rem; width: 100%; max-width: 24rem; padding: .2rem; border-radius: var(--radius-control); background: var(--surface-raised); }
.catalog-experience__modes :deep(button) { flex: 1; justify-content: center; min-height: 44px; border-radius: var(--radius-control); }
.catalog-experience--transitioning { --catalog-entry-delay: 320ms; }
.catalog-mode-enter-active, .catalog-mode-leave-active { transition: opacity 160ms ease, transform 160ms ease; }
.catalog-mode-enter-from { opacity: 0; transform: translateY(8px); }
.catalog-mode-leave-to { opacity: 0; transform: translateY(-4px); }
@media (prefers-reduced-motion: reduce) { .catalog-mode-enter-active, .catalog-mode-leave-active { transition: opacity 80ms ease; } .catalog-mode-enter-from, .catalog-mode-leave-to { transform: none; } }
</style>
