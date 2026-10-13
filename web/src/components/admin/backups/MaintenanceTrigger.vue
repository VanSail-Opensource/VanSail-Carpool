<script setup lang="ts">
import { computed, shallowRef } from 'vue'
import { motion } from 'motion-v'

import { api } from '@/api/client'
import type { OperationReceipt } from '@/api/types'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import OperationStatusNotice from '@/components/common/OperationStatusNotice.vue'
import { useDurableCommand } from '@/composables/useDurableCommand'
import { useMotionPreferences } from '@/composables/useMotionPreferences'
import { useI18n } from '@/i18n'

const emit = defineEmits<{ completed: [] }>()
const { t } = useI18n()
const confirmationOpen = shallowRef(false)
const { reducedMotion } = useMotionPreferences()
const maintenance = useDurableCommand({
  errorKey: 'errors.adminAction',
  onTerminal: (receipt: OperationReceipt) => {
    if (receipt.status === 'succeeded') emit('completed')
  },
})

const statusMessage = computed(() => {
  const status = maintenance.receipt.value?.status
  if (status === 'succeeded') return t('adminBackups.maintenanceSucceeded')
  if (status === 'failed') return t('adminBackups.maintenanceFailed')
  return status ? t('adminBackups.maintenanceQueued') : null
})

function showConfirmation(): void {
  confirmationOpen.value = true
}

function queueMaintenance(): void {
  confirmationOpen.value = false
  void maintenance.execute('maintenance', 'retention', (key) => api.runAdminMaintenance(key))
}
</script>

<template>
  <motion.div class="maintenance-trigger" layout :transition="{ duration: reducedMotion ? 0.08 : 0.16, ease: 'easeOut' }">
    <UButton
      icon="i-ph-broom"
      color="neutral"
      variant="outline"
      :disabled="maintenance.busy.value"
      :loading="maintenance.busy.value"
      :label="t('adminBackups.maintenance')"
      @click="showConfirmation"
    />
    <OperationStatusNotice
      :receipt="maintenance.receipt.value"
      :error="maintenance.error.value"
      :checking="maintenance.checking.value"
      :message="statusMessage"
      @refresh="maintenance.refresh"
    />
    <ConfirmDialog
      v-model:open="confirmationOpen"
      :title="t('adminBackups.maintenanceTitle')"
      :description="t('adminBackups.maintenanceDescription')"
      :confirm-label="t('adminBackups.maintenanceConfirm')"
      :busy="maintenance.submitting.value"
      danger
      @confirm="queueMaintenance"
    />
  </motion.div>
</template>

<style scoped>
.maintenance-trigger { display: grid; gap: 0.65rem; }
.maintenance-trigger > :deep(button) { justify-self: start; }
@media (max-width: 639px) {
  .maintenance-trigger > :deep(button) { width: 100%; }
}
</style>
