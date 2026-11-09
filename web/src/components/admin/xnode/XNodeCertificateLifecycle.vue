<script setup lang="ts">
import { computed, watch } from 'vue'
import { localizedError, useI18n } from '@/i18n'
import OperationStatusNotice from '@/components/common/OperationStatusNotice.vue'
import { useCertificateLifecycle } from './useCertificateLifecycle'

const props = defineProps<{ nodeId: string; profileId: string; revision: number; canRun: boolean; active: boolean; reason?: string }>()
const emit = defineEmits<{ busy: [value: boolean] }>()
const { t } = useI18n()
const lifecycle = useCertificateLifecycle({ nodeId: () => props.nodeId, profileId: () => props.profileId, revision: () => props.revision, canRun: () => props.canRun })
const { receipt, checking, busy, error, recovering, blocksMutations, uncertain, canRetry, lastAction } = lifecycle
const action = computed(() => props.active ? 'renew' : 'issue')
const message = computed(() => receipt.value?.errorCode ? localizedError({ code: receipt.value.errorCode }, 'certificates.actionFailed') : null)
watch(blocksMutations, value => emit('busy', value), { immediate: true })
</script>
<template>
  <div class="grid min-w-0 gap-3" data-certificate-lifecycle>
    <p v-if="reason" class="text-xs text-muted">{{ reason }}</p>
    <p v-if="uncertain" role="status" class="text-xs text-warning">{{ t('certificates.uncertain') }}</p>
    <div class="flex flex-wrap gap-2">
      <UButton type="button" color="neutral" variant="outline" :label="t(`certificates.${action}`)" :loading="busy" :disabled="!canRun || blocksMutations" @click="lifecycle.run(action)" />
      <UButton v-if="canRetry && lastAction" type="button" color="neutral" variant="outline" :label="t('common.tryAgain')" :disabled="!canRun || blocksMutations" @click="lifecycle.run(lastAction!)" />
      <UButton type="button" color="neutral" variant="ghost" :label="t('operations.checkStatus')" :loading="recovering || checking" :disabled="busy && !receipt" @click="lifecycle.refresh" />
    </div>
    <OperationStatusNotice :receipt="receipt" :message="message" :error="error" :checking="checking || recovering" @refresh="lifecycle.refresh" />
  </div>
</template>
