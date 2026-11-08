<script setup lang="ts">
import { defineAsyncComponent, onErrorCaptured, shallowRef } from 'vue'
import { useI18n } from '@/i18n'
import { selectionHaptic } from '@/utils/telegram'
import { isRouteChunkError } from '@/router/recovery'
import CatalogChunkState from './CatalogChunkState.vue'

const HourlyPage = defineAsyncComponent({ loader: () => import('@/components/hourly/HourlyPage.vue'), loadingComponent: CatalogChunkState, errorComponent: CatalogChunkState, delay: 0 })
const PAYGPage = defineAsyncComponent({ loader: () => import('./PAYGPage.vue'), loadingComponent: CatalogChunkState, errorComponent: CatalogChunkState, delay: 0 })
onErrorCaptured(error => isRouteChunkError(error) ? false : undefined)

const { t } = useI18n()
const mode = shallowRef<'payg' | 'combo'>('payg')

function selectMode(next: 'payg' | 'combo'): void {
  if (mode.value === next) return
  selectionHaptic()
  mode.value = next
}
</script>

<template>
  <div class="page catalog-experience">
    <header class="catalog-experience__header">
      <h1 class="sr-only">{{ t(mode === 'payg' ? 'payg.title' : 'hourly.title') }}</h1>
      <div class="catalog-experience__modes" role="group" :aria-label="t('payg.mode')">
        <UButton color="neutral" :variant="mode === 'payg' ? 'solid' : 'ghost'" :aria-pressed="mode === 'payg'" :label="t('payg.title')" @click="selectMode('payg')" />
        <UButton color="neutral" :variant="mode === 'combo' ? 'solid' : 'ghost'" :aria-pressed="mode === 'combo'" :label="t('payg.combo')" @click="selectMode('combo')" />
      </div>
    </header>
    <div :key="mode" class="catalog-experience__content">
      <PAYGPage v-if="mode === 'payg'" />
      <HourlyPage v-else />
    </div>
  </div>
</template>

<style scoped>
.catalog-experience__header { margin-bottom: .15rem; }
.catalog-experience__modes { display: flex; gap: .2rem; width: 100%; max-width: 24rem; padding: .2rem; border-radius: var(--radius-control); background: var(--surface-raised); }
.catalog-experience__modes :deep(button) { flex: 1; justify-content: center; min-height: 44px; border-radius: var(--radius-control); }
.catalog-experience__content { animation: catalog-content-enter 140ms ease-out both; }
@keyframes catalog-content-enter { from { opacity: 0; transform: translateY(6px); } to { opacity: 1; transform: translateY(0); } }
@media (prefers-reduced-motion: reduce) { .catalog-experience__content { animation: none; } }
</style>
