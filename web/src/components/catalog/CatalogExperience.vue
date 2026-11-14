<script setup lang="ts">
import { defineAsyncComponent, onErrorCaptured, onMounted, onScopeDispose, shallowRef } from 'vue'
import { useI18n } from '@/i18n'
import { selectionHaptic } from '@/utils/telegram'
import { isRouteChunkError } from '@/router/recovery'
import CatalogChunkState from './CatalogChunkState.vue'
import { catalogPages, preloadCatalogPages } from './catalogPages'

const HourlyPage = defineAsyncComponent({ loader: catalogPages.hourly, loadingComponent: CatalogChunkState, errorComponent: CatalogChunkState, delay: 0 })
const PAYGPage = defineAsyncComponent({ loader: catalogPages.traffic, loadingComponent: CatalogChunkState, errorComponent: CatalogChunkState, delay: 0 })
const loadBillingDemo = () => import('./BillingDemo.vue')
const BillingDemo = defineAsyncComponent({ loader: loadBillingDemo, loadingComponent: CatalogChunkState, errorComponent: CatalogChunkState, delay: 80 })
onErrorCaptured(error => isRouteChunkError(error) ? false : undefined)

const { t } = useI18n()
const mode = shallowRef<'payg' | 'combo'>('payg')
const demoOpen = shallowRef(false)
const demoTrigger = shallowRef<globalThis.HTMLElement | null>(null)
function openDemo(event: globalThis.MouseEvent): void {
  demoTrigger.value = event.currentTarget instanceof globalThis.HTMLElement ? event.currentTarget : null
  demoOpen.value = true
}
function preloadDemo(): void { void loadBillingDemo().catch(() => undefined) }
let idle: number | undefined
let timer: ReturnType<typeof globalThis.setTimeout> | undefined
onMounted(() => {
  const warm = () => { void preloadCatalogPages() }
  if (globalThis.requestIdleCallback) idle = globalThis.requestIdleCallback(warm, { timeout: 1200 })
  else timer = globalThis.setTimeout(warm, 100)
})
onScopeDispose(() => {
  if (idle !== undefined) globalThis.cancelIdleCallback?.(idle)
  if (timer !== undefined) globalThis.clearTimeout(timer)
})

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
        <UButton color="neutral" icon="i-ph-arrows-down-up" :variant="mode === 'payg' ? 'solid' : 'ghost'" :aria-pressed="mode === 'payg'" :label="t('payg.title')" @click="selectMode('payg')" />
        <UButton color="neutral" icon="i-ph-clock" :variant="mode === 'combo' ? 'solid' : 'ghost'" :aria-pressed="mode === 'combo'" :label="t('payg.combo')" @click="selectMode('combo')" />
      </div>
      <UButton class="catalog-experience__demo" color="neutral" variant="ghost" icon="i-ph-play-circle" :label="t('payg.demo.open')" :aria-label="t('payg.demo.openFor', { mode: t(mode === 'payg' ? 'payg.title' : 'hourly.title') })" data-billing-demo @pointerenter="preloadDemo" @focus="preloadDemo" @click="openDemo" />
    </header>
    <div :key="mode" class="catalog-experience__content">
      <PAYGPage v-if="mode === 'payg'" />
      <HourlyPage v-else />
    </div>
    <BillingDemo v-if="demoOpen" v-model:open="demoOpen" :mode="mode === 'payg' ? 'traffic' : 'hourly'" :return-focus="demoTrigger" />
  </div>
</template>

<style scoped>
.catalog-experience { min-width: 0; }
.catalog-experience__header { display: flex; flex-wrap: wrap; align-items: center; gap: .5rem; margin-bottom: .65rem; }
.catalog-experience__modes { display: flex; flex: 1 1 20rem; gap: .2rem; min-width: 0; max-width: 26rem; padding: .2rem; border-radius: var(--radius-control); background: var(--surface-raised); }
.catalog-experience__modes :deep(button) { flex: 1; justify-content: center; min-height: 44px; border-radius: var(--radius-control); }
.catalog-experience__demo { min-height: 44px; margin-inline-start: auto; color: var(--accent); }
.catalog-experience__content { animation: catalog-content-enter 140ms ease-out both; }
@keyframes catalog-content-enter { from { opacity: 0; transform: translateY(6px); } to { opacity: 1; transform: translateY(0); } }
@media (prefers-reduced-motion: reduce) { .catalog-experience__content { animation: none; } }
@media (max-width: 480px) { .catalog-experience__modes { flex-basis: 100%; max-width: none; } .catalog-experience__demo { min-height: 36px; padding-block: .25rem; } }
</style>
