<script setup lang="ts">
import { computed, shallowRef, watch } from 'vue'
import { AnimatePresence, motion } from 'motion-v'

import type { OperationReceipt, OperationResolution } from '@/api/adminOperations'
import InlineNotice from '@/components/common/InlineNotice.vue'
import { useMotionPreferences } from '@/composables/useMotionPreferences'
import { useI18n } from '@/i18n'

const props = defineProps<{ open: boolean; operation: OperationReceipt | null; busy: boolean; error?: string | null }>()
const emit = defineEmits<{
  'update:open': [value: boolean]
  resolve: [payload: { resolution: OperationResolution; reason: string }]
}>()
const { t } = useI18n()
const resolution = shallowRef<OperationResolution>('succeeded')
const reason = shallowRef('')
const resolutionItems = computed(() => {
  const values: OperationResolution[] = ['succeeded', 'failed', 'compensated', 'partial']
  return values.filter((value) => props.operation?.status !== 'partial' || value !== 'partial')
    .map((value) => ({ value, label: t(`adminUserProfile.operationStatus.${value}`) }))
})
const canResolve = computed(() => reason.value.trim().length >= 3 && resolutionItems.value.some((item) => item.value === resolution.value))
const workflowState = computed(() => props.busy ? 'processing' : props.error ? `error:${props.error}` : 'review')
const { reducedMotion } = useMotionPreferences()

watch(() => props.open, (open) => {
  if (!open) return
  resolution.value = 'succeeded'
  reason.value = ''
})

function submit(): void {
  if (!canResolve.value) return
  emit('resolve', { resolution: resolution.value, reason: reason.value.trim() })
}
</script>

<template>
  <UModal :open="open" :title="t('adminUserProfile.resolveTitle')" :description="t('adminUserProfile.resolveHint')" :dismissible="!busy" :close="false" :ui="{ header: 'admin-overlay-header', wrapper: 'admin-overlay-copy', footer: 'justify-end' }" @update:open="emit('update:open', $event)">
    <template #body>
      <AnimatePresence mode="wait" :initial="false">
        <motion.div :key="workflowState" :initial="reducedMotion ? { opacity: 0 } : { opacity: 0, y: 4 }" :animate="{ opacity: 1, y: 0 }" :exit="{ opacity: 0 }" :transition="{ duration: reducedMotion ? 0.08 : 0.18, ease: 'easeOut' }">
          <UAlert color="warning" variant="soft" icon="i-ph-warning-circle" :title="t('adminUserProfile.resolveWarning')" :description="t('adminUserProfile.resolveWarningHint')" />
          <UForm id="operation-resolution" :state="{ resolution, reason }" class="form-stack" @submit="submit">
            <UFormField name="resolution" :label="t('adminUserProfile.resolution')" required><USelect v-model="resolution" class="w-full" :items="resolutionItems" value-key="value" /></UFormField>
            <UFormField name="reason" :label="t('adminReason.reason')" required><UTextarea v-model.trim="reason" :rows="3" :minlength="3" :maxlength="500" /></UFormField>
            <InlineNotice v-if="error" tone="warning">{{ error }}</InlineNotice>
          </UForm>
        </motion.div>
      </AnimatePresence>
    </template>
    <template #footer>
      <UButton color="neutral" variant="outline" :label="t('common.cancel')" :disabled="busy" @click="emit('update:open', false)" />
      <UButton type="submit" form="operation-resolution" color="warning" icon="i-ph-gavel" :label="busy ? t('common.working') : t('adminUserProfile.recordResolution')" :loading="busy" :disabled="!canResolve || busy" />
    </template>
  </UModal>
</template>
