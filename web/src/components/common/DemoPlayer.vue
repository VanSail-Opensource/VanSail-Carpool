<script setup lang="ts">
import { computed } from 'vue'
import { useIllustrationPlayback } from '@/composables/useIllustrationPlayback'
import { useTelegramBackButton } from '@/composables/useTelegramBackButton'
import { useI18n } from '@/i18n'
import DemoTransport from './DemoTransport.vue'

const open = defineModel<boolean>('open', { required: true })
const props = defineProps<{
  title: string
  description: string
  chapters: readonly { icon: string; label: string; caption?: string; detail?: string }[]
  interval?: number
  resetKey?: string
  returnFocus?: globalThis.HTMLElement | null
  contentClass?: string
  billingMode?: string
}>()
const emit = defineEmits<{ replay: [] }>()
const { t } = useI18n()
const playback = useIllustrationPlayback({ active: () => open.value, stepCount: () => props.chapters.length, resetKey: () => props.resetKey, interval: props.interval })
const { step, phase, progress, elapsed, duration, playing, reducedMotion } = playback
const current = computed(() => props.chapters[step.value] ?? props.chapters[0])
useTelegramBackButton(computed(() => open.value), () => { open.value = false })
function restoreFocus(event: globalThis.Event): void {
  if (!props.returnFocus?.isConnected) return
  event.preventDefault()
  props.returnFocus.focus({ preventScroll: true })
}
function replay(): void { emit('replay'); playback.replay() }
</script>

<template>
  <UModal v-model:open="open" :title="title" :description="description" :content="{ onCloseAutoFocus: restoreFocus }" :transition="!reducedMotion" :close="{ 'aria-label': t('common.close') }" :ui="{ content: 'sm:max-w-2xl', body: 'p-4 sm:p-6', description: 'sr-only' }">
    <template #body>
      <div class="demo-player" :class="contentClass" :data-demo-playing="playing" :data-billing-mode="billingMode">
        <slot :step="step" :phase="phase" :reduced-motion="reducedMotion" :playing="playing" :select-step="playback.selectStep" />
        <p v-if="current" class="demo-player__caption"><UIcon :name="current.icon" aria-hidden="true" /><span>{{ current.caption ?? current.label }}</span></p>
        <p class="sr-only" role="status" aria-live="polite" aria-atomic="true">{{ current?.detail ?? current?.caption ?? current?.label }}</p>
        <slot name="controls" :step="step" :phase="phase" :reduced-motion="reducedMotion" :select-step="playback.selectStep" />
        <DemoTransport :chapters="chapters" :step="step" :progress="progress" :elapsed="elapsed" :duration="duration" :playing="playing" :reduced-motion="reducedMotion" @select="playback.selectStep" @seek="playback.seek" @toggle="playback.toggle" @replay="replay" />
        <slot name="details" :step="step" />
      </div>
    </template>
  </UModal>
</template>

<style scoped>
.demo-player { display: grid; gap: .75rem; min-width: 0; }
.demo-player__caption { display: flex; align-items: center; justify-content: center; gap: .5rem; margin: 0; min-height: 2.8rem; color: var(--text); font-size: .85rem; line-height: 1.5; text-align: center; }
.demo-player__caption .iconify { flex: none; color: var(--accent); font-size: 1.2rem; }
</style>
