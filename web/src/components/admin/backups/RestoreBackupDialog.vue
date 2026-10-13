<script setup lang="ts">
import { computed, shallowRef, watch } from 'vue'
import { AnimatePresence, motion } from 'motion-v'

import { useI18n } from '@/i18n'
import { useMotionPreferences } from '@/composables/useMotionPreferences'

const props = defineProps<{ open: boolean; backupName: string; busy: boolean; error?: string | null }>()
const emit = defineEmits<{ 'update:open': [value: boolean]; restore: [payload: { reason: string; confirmation: string }] }>()
const { t } = useI18n()
const reason = shallowRef('')
const confirmation = shallowRef('')
const requiredConfirmation = computed(() => `${t('restoreBackup.confirmationPrefix')} ${props.backupName}`)
const canRestore = computed(() => reason.value.trim().length >= 4 && confirmation.value === requiredConfirmation.value)
const workflowState = computed(() => props.busy ? 'restoring' : canRestore.value ? 'confirm' : 'warning')
const { reducedMotion } = useMotionPreferences()

watch(() => props.open, (open) => {
  if (open) { reason.value = ''; confirmation.value = '' }
})
</script>

<template>
  <UModal :open="open" :title="t('restoreBackup.title')" :description="t('restoreBackup.copy')" :dismissible="!busy" :close="false" :ui="{ header: 'admin-overlay-header', wrapper: 'admin-overlay-copy', footer: 'justify-end' }" @update:open="emit('update:open', $event)">
    <template #body>
      <UAlert v-if="error" color="warning" variant="soft" :title="error" />
      <AnimatePresence mode="wait" :initial="false">
        <motion.div :key="workflowState" :initial="reducedMotion ? { opacity: 0 } : { opacity: 0, y: 4 }" :animate="{ opacity: 1, y: 0 }" :exit="{ opacity: 0 }" :transition="{ duration: reducedMotion ? 0.08 : 0.18, ease: 'easeOut' }">
          <UIcon name="i-ph-warning-fill" class="dialog-icon dialog-icon--danger" aria-hidden="true" />
          <UFormField name="reason" :label="t('adminReason.reason')" required><UTextarea v-model.trim="reason" :rows="3" :minlength="4" :maxlength="300" /></UFormField>
          <UFormField name="confirmation" :label="t('databaseRecord.typeConfirmation', { confirmation: requiredConfirmation })" required><UInput v-model="confirmation" autocomplete="off" /></UFormField>
        </motion.div>
      </AnimatePresence>
    </template>
    <template #footer>
      <UButton color="neutral" variant="outline" :label="t('common.cancel')" :disabled="busy" @click="emit('update:open', false)" />
      <UButton color="error" :disabled="busy || !canRestore" :loading="busy" :label="busy ? t('restoreBackup.staging') : t('restoreBackup.confirm')" @click="emit('restore', { reason: reason.trim(), confirmation })" />
    </template>
  </UModal>
</template>
