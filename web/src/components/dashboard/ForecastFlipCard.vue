<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, shallowRef, useTemplateRef, watch } from 'vue'
import { motion } from 'motion-v'

import type { Purchase } from '@/api/types'
import { useTelegramBackButton } from '@/composables/useTelegramBackButton'
import { motionSpring, reducedMotionTransition } from '@/composables/motionPresets'
import { useMotionPreferences } from '@/composables/useMotionPreferences'
import RideSummaryFace from './RideSummaryFace.vue'
import PersonalPredictions from './PersonalPredictions.vue'

const props = defineProps<{
  active: Purchase
  squadNames?: readonly string[]
}>()

const flipped = shallowRef(false)
const cardHeight = shallowRef<string | null>(null)
const summaryFace = useTemplateRef<globalThis.HTMLElement>('summaryFace')
const predictionsFace = useTemplateRef<globalThis.HTMLElement>('predictionsFace')
const flipInnerStyle = computed(() => cardHeight.value ? { height: cardHeight.value } : undefined)
const ownsTelegramBackButton = computed(() => flipped.value)
const { reducedMotion } = useMotionPreferences()
let changingFace = false
let resizeObserver: globalThis.ResizeObserver | undefined
let animationFrame: number | undefined
let frameTimer: ReturnType<typeof globalThis.setTimeout> | undefined
let resolveFrame: (() => void) | undefined
let unmounted = false

function activeFace(): globalThis.HTMLElement | null {
  return flipped.value ? predictionsFace.value : summaryFace.value
}

function syncHeight(): void {
  if (unmounted) return
  const height = activeFace()?.offsetHeight ?? 0
  if (height > 0) cardHeight.value = `${height}px`
}

function nextFrame(): Promise<void> {
  return new Promise((resolve) => {
    const finish = (): void => {
      animationFrame = undefined
      frameTimer = undefined
      resolveFrame = undefined
      resolve()
    }
    resolveFrame = finish
    if (typeof globalThis.requestAnimationFrame === 'function') {
      animationFrame = globalThis.requestAnimationFrame(finish)
      return
    }
    frameTimer = globalThis.setTimeout(finish, 0)
  })
}

async function changeFace(next: boolean): Promise<void> {
  if (changingFace || flipped.value === next) return
  changingFace = true
  try {
    syncHeight()
    await nextFrame()
    if (unmounted) return
    flipped.value = next
    await nextTick()
    syncHeight()
    activeFace()?.querySelector<globalThis.HTMLButtonElement>('button')?.focus()
  } finally {
    changingFace = false
  }
}

async function showDetail(): Promise<void> {
  if (flipped.value || changingFace) return
  await changeFace(true)
}

async function showSummary(): Promise<void> {
  if (!flipped.value || changingFace) return
  await changeFace(false)
}

useTelegramBackButton(ownsTelegramBackButton, showSummary)

watch(() => props.active.id, () => {
  flipped.value = false
  cardHeight.value = null
})


onMounted(() => {
  syncHeight()
  if (typeof globalThis.ResizeObserver !== 'function') return
  resizeObserver = new globalThis.ResizeObserver(syncHeight)
  if (summaryFace.value) resizeObserver.observe(summaryFace.value)
  if (predictionsFace.value) resizeObserver.observe(predictionsFace.value)
})

onUnmounted(() => {
  unmounted = true
  resizeObserver?.disconnect()
  if (animationFrame !== undefined && typeof globalThis.cancelAnimationFrame === 'function') globalThis.cancelAnimationFrame(animationFrame)
  if (frameTimer !== undefined) globalThis.clearTimeout(frameTimer)
  resolveFrame?.()
})
</script>

<template>
  <div class="home-ride__summary home-ride__flip-card" :class="{ 'is-flipped': flipped }">
    <motion.div
      class="home-ride__flip-inner"
      :style="flipInnerStyle"
      layout
      :animate="{ rotateY: reducedMotion ? 0 : flipped ? 180 : 0 }"
      :transition="reducedMotion ? reducedMotionTransition : motionSpring"
    >
      <div ref="summaryFace" class="home-ride__flip-face home-ride__flip-face--front" :aria-hidden="flipped" :inert="flipped">
        <UButton
          id="your-ride-summary"
          class="home-ride__summary-trigger"
          type="button"
          block
          color="neutral"
          variant="ghost"
          aria-controls="your-ride-predictions"
          :aria-expanded="flipped"
          :aria-label="$t('predictions.open')"
          :aria-hidden="flipped"
          :inert="flipped"
          data-haptic="open"
          @click="showDetail"
        />
        <RideSummaryFace
          class="home-ride__summary-content"
          :active="active"
          :squad-names="squadNames"
          @open-predictions="showDetail"
        />
      </div>
      <div id="your-ride-predictions" ref="predictionsFace" class="home-ride__flip-face home-ride__flip-face--back" :aria-hidden="!flipped" :inert="!flipped">
        <PersonalPredictions v-if="flipped" back @back="showSummary" />
      </div>
    </motion.div>
  </div>
</template>
