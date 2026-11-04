<script setup lang="ts">
import type { IPBlock, OperationReceipt } from '@/api/types'
import OperationStatusNotice from '@/components/common/OperationStatusNotice.vue'
import { t } from '@/i18n'
import { formatDateTime } from '@/utils/format'
const open = defineModel<boolean>('open', { required: true })
defineProps<{ block: IPBlock | null; receipt?: OperationReceipt | null; busy?: boolean; checking?: boolean; error?: string | null }>()
const emit = defineEmits<{ confirm: []; refresh: [] }>()
</script>

<template>
  <UModal v-model:open="open" :title="t('adminUserProfile.unblockIP')" :dismissible="!busy">
    <template #body>
      <div v-if="block" class="grid gap-3">
        <code class="break-all">{{ block.ip }}</code>
        <p class="text-sm text-muted">{{ formatDateTime(block.expiresAt) }}</p>
        <OperationStatusNotice :receipt="receipt ?? null" :error="error ?? null" :checking="checking" @refresh="emit('refresh')" />
      </div>
    </template>
    <template #footer>
      <UButton color="neutral" variant="outline" :disabled="busy" :label="t('common.close')" @click="open = false" />
      <UButton v-if="!receipt" :loading="busy" :disabled="busy || !block" :label="t('adminUserProfile.unblockIP')" @click="emit('confirm')" />
    </template>
  </UModal>
</template>
