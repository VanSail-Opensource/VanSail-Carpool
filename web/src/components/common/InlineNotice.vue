<script setup lang="ts">
import { computed } from 'vue'
import { AnimatePresence, motion } from 'motion-v'

import { useMotionPreferences } from '@/composables/useMotionPreferences'

defineOptions({ inheritAttrs: false })

const props = withDefaults(defineProps<{
  tone?: 'info' | 'warning' | 'success'
  title?: string
}>(), {
  tone: 'info',
  title: undefined,
})

const presentation = computed(() => ({
  info: { icon: 'i-ph-info-fill', color: 'info' },
  warning: { icon: 'i-ph-warning-circle-fill', color: 'warning' },
  success: { icon: 'i-ph-check-circle-fill', color: 'success' },
} as const)[props.tone])
const { reducedMotion, offset } = useMotionPreferences()
</script>

<template>
  <AnimatePresence :initial="false">
    <motion.div
      v-bind="$attrs"
      key="notice"
      class="notice-motion"
      :initial="{ opacity: 0, y: offset(-5) }"
      :animate="{ opacity: 1, y: 0 }"
      :exit="{ opacity: 0, y: reducedMotion ? 0 : -5 }"
      :transition="{ duration: reducedMotion ? 0.08 : 0.2, ease: 'easeOut' }"
    >
      <UAlert
        class="notice"
        :class="`notice--${tone}`"
        :icon="presentation.icon"
        :color="presentation.color"
        variant="soft"
        :title="title"
        role="status"
      >
        <template #description><slot /></template>
      </UAlert>
    </motion.div>
  </AnimatePresence>
</template>
