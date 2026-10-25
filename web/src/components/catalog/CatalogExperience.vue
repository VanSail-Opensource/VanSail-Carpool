<script setup lang="ts">
import { computed, shallowRef, watch } from 'vue'
import { useI18n } from '@/i18n'
import { useMotionPreferences } from '@/composables/useMotionPreferences'
import { selectionHaptic } from '@/utils/telegram'
import CatalogPage from './CatalogPage.vue'
import PAYGPage from './PAYGPage.vue'
import CatalogModeTransition from './CatalogModeTransition.vue'

const { t } = useI18n()
const mode = shallowRef<'payg' | 'combo'>('payg')
const transitioning = shallowRef(false)
const ready = shallowRef(false)
const { reducedMotion } = useMotionPreferences()
const hideCombo = computed(() => mode.value === 'combo' && transitioning.value && !reducedMotion.value)
watch(mode, () => { ready.value = false; transitioning.value = !reducedMotion.value }, { flush: 'sync' })

function selectMode(next: 'payg' | 'combo'): void {
  if (mode.value === next) return
  selectionHaptic()
  mode.value = next
}
</script>

<template>
  <div class="page catalog-experience" :class="{ 'catalog-experience--transitioning': transitioning }">
    <CatalogModeTransition :mode="mode" :ready="ready" @active="transitioning = $event" />
    <header class="catalog-experience__header">
      <h1 class="sr-only">{{ t(mode === 'payg' ? 'payg.title' : 'catalog.title') }}</h1>
      <div class="catalog-experience__modes" role="group" :aria-label="t('payg.mode')">
        <UButton color="neutral" :variant="mode === 'payg' ? 'solid' : 'ghost'" :aria-pressed="mode === 'payg'" :label="t('payg.title')" @click="selectMode('payg')" />
        <UButton color="neutral" :variant="mode === 'combo' ? 'solid' : 'ghost'" :aria-pressed="mode === 'combo'" :label="t('payg.combo')" @click="selectMode('combo')" />
      </div>
    </header>
    <div :key="mode" class="catalog-experience__content" :class="{ 'catalog-experience__content--hidden': hideCombo, 'catalog-experience__content--entering': mode === 'combo' && !hideCombo }" :inert="hideCombo" :aria-hidden="hideCombo || undefined">
      <PAYGPage v-if="mode === 'payg'" @ready="ready = $event" />
      <CatalogPage v-else embedded @ready="ready = $event" />
    </div>
  </div>
</template>

<style scoped>
.catalog-experience__header { margin-bottom: .15rem; }
.catalog-experience__modes { display: flex; gap: .2rem; width: 100%; max-width: 24rem; padding: .2rem; border-radius: var(--radius-control); background: var(--surface-raised); }
.catalog-experience__modes :deep(button) { flex: 1; justify-content: center; min-height: 44px; border-radius: var(--radius-control); }
.catalog-experience--transitioning { --catalog-entry-delay: 320ms; }
.catalog-experience__content--hidden { visibility: hidden; pointer-events: none; }
.catalog-experience__content--entering { animation: catalog-content-enter 200ms ease-out both; }
@keyframes catalog-content-enter { from { opacity: 0; transform: translateY(6px); } to { opacity: 1; transform: translateY(0); } }
@media (prefers-reduced-motion: reduce) { .catalog-experience__content--entering { animation: none; } }
</style>
