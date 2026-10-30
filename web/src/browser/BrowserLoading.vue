<script setup lang="ts">
import { shallowRef, watch } from 'vue'
import { useI18n } from '@/i18n'
import { useMotionPreferences } from '@/composables/useMotionPreferences'
import { usePendingStatus } from '@/composables/usePendingStatus'

const props = defineProps<{ status?: string; error?: string | null; failureTitle?: string }>()
const { t } = useI18n()
const { slow } = usePendingStatus(() => !props.error)
const { reducedMotion } = useMotionPreferences()
const animate = shallowRef(true)
watch(() => reducedMotion.value || !!props.error, stop => { if (stop) animate.value = false }, { immediate: true })
</script>

<template>
  <main class="browser-entry" :class="{ 'browser-entry--animate': animate, 'browser-entry--preferences': $slots.preferences }">
    <div v-if="$slots.preferences" class="browser-entry__preferences"><slot name="preferences" /></div>
    <div class="browser-entry__content">
      <h1 class="browser-entry__identity">{{ t('app.name') }}</h1>
      <div class="browser-entry__status" role="status" aria-live="polite" aria-atomic="true">
        <template v-if="error">
          <p class="browser-entry__failure">{{ failureTitle ?? t('browser.startupFailed') }}</p>
          <p v-if="error !== (failureTitle ?? t('browser.startupFailed'))">{{ error }}</p>
        </template>
        <p v-else>{{ status ?? t(slow ? 'browser.loadingWorkspaceSlow' : 'browser.loadingWorkspace') }}</p>
      </div>
      <div v-if="!error" class="loading-track" aria-hidden="true" />
      <div v-else class="browser-entry__actions"><slot /></div>
    </div>
  </main>
</template>
