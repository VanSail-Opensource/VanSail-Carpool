<script setup lang="ts">
import { LayoutGroup, motion } from 'motion-v'

import type { EmbyLibrary } from '@/api/features'
import { motionSpring } from '@/composables/motionPresets'
import { useMotionPreferences } from '@/composables/useMotionPreferences'
import { selectionHaptic } from '@/utils/telegram'

const props = defineProps<{ libraries: readonly EmbyLibrary[]; selectedIds: readonly string[]; disabled?: boolean }>()
const emit = defineEmits<{ toggle: [id: string] }>()
const { reducedMotion } = useMotionPreferences()

function toggle(id: string): void {
  selectionHaptic()
  emit('toggle', id)
}
</script>

<template>
  <fieldset class="emby-library-picker">
    <legend>{{ $t('emby.disabledLibraries') }}</legend>
    <LayoutGroup>
      <motion.label
        v-for="library in libraries"
        :key="library.id"
        class="emby-library"
        :class="{ 'emby-library--selected': props.selectedIds.includes(library.id) }"
        layout
      >
        <motion.span
          v-if="props.selectedIds.includes(library.id)"
          layout-id="emby-library-selection"
          class="emby-library__selected-indicator"
          :transition="reducedMotion ? { duration: 0.08 } : motionSpring"
          aria-hidden="true"
        />
        <span class="feature-icon feature-icon--small"><UIcon name="i-ph-film-slate" /></span>
        <span class="emby-library__name">{{ library.name }}</span>
        <UCheckbox
          :model-value="props.selectedIds.includes(library.id)"
          :disabled="disabled"
          :aria-label="library.name"
          @update:model-value="toggle(library.id)"
        />
      </motion.label>
    </LayoutGroup>
    <small class="field-hint">{{ $t('emby.disabledLibrariesHint') }}</small>
  </fieldset>
</template>

<style scoped>
.emby-library-picker { display: grid; gap: 0.5rem; margin: 0; padding: 0; border: 0; }
.emby-library-picker legend { margin-bottom: 0.4rem; color: var(--text-muted); font-size: 0.78rem; font-weight: 700; }
.emby-library { position: relative; min-height: 54px; display: grid; grid-template-columns: auto minmax(0, 1fr) auto; align-items: center; gap: 0.65rem; padding: 0.5rem; border: 1px solid var(--line); border-radius: var(--radius-control); background: var(--surface-raised); font-size: 0.8rem; font-weight: 700; isolation: isolate; }
.emby-library > :not(.emby-library__selected-indicator) { position: relative; z-index: 1; }
.emby-library__name { min-width: 0; overflow-wrap: anywhere; }
.emby-library__selected-indicator { position: absolute; inset: 0; z-index: 0; border: 1px solid var(--accent); border-radius: inherit; background: var(--accent-soft); pointer-events: none; }
</style>
