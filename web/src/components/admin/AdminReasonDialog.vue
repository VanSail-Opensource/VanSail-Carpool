<script setup lang="ts">
import { computed } from 'vue'
import { AnimatePresence, motion } from 'motion-v'

import InlineNotice from '@/components/common/InlineNotice.vue'
import { useMotionPreferences } from '@/composables/useMotionPreferences'
import { useI18n } from '@/i18n'

const open = defineModel<boolean>('open', { required: true })
const reason = defineModel<string>('reason', { required: true })

const props = withDefaults(defineProps<{
  title: string
  description: string
  confirmLabel: string
  busy?: boolean
  danger?: boolean
  error?: string | null
}>(), { busy: false, danger: false, error: null })

defineEmits<{ confirm: [] }>()
const { t } = useI18n()
const workflowState = computed(() => props.busy ? 'processing' : props.error ? `error:${props.error}` : 'reason')
const { reducedMotion } = useMotionPreferences()
</script>

<template>
  <UModal v-model:open="open" :title="title" :description="description" :dismissible="!busy" :close="false" :ui="{ header: 'admin-overlay-header', wrapper: 'admin-overlay-copy', footer: 'justify-end' }">
    <template #body>
      <AnimatePresence mode="wait" :initial="false">
        <motion.div
          :key="workflowState"
          :initial="reducedMotion ? { opacity: 0 } : { opacity: 0, y: 4 }"
          :animate="{ opacity: 1, y: 0 }"
          :exit="{ opacity: 0 }"
          :transition="{ duration: reducedMotion ? 0.08 : 0.18, ease: 'easeOut' }"
        >
          <UFormField name="reason" :label="t('adminReason.reason')" required>
            <UTextarea v-model.trim="reason" :rows="3" :minlength="4" :maxlength="300" :placeholder="t('adminReason.placeholder')" />
          </UFormField>
          <InlineNotice v-if="error" tone="warning">{{ error }}</InlineNotice>
        </motion.div>
      </AnimatePresence>
    </template>
    <template #footer="{ close }">
      <UButton color="neutral" variant="outline" :label="t('common.cancel')" :disabled="busy" @click="close" />
      <UButton :color="danger ? 'error' : 'primary'" trailing-icon="i-ph-arrow-right" :disabled="reason.length < 4 || busy" :loading="busy" :label="busy ? t('adminReason.working') : confirmLabel" @click="$emit('confirm')" />
    </template>
  </UModal>
</template>
