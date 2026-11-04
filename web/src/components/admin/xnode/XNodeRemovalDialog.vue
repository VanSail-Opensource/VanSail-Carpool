<script setup lang="ts">
import { computed } from 'vue'
import { xnodeApi, type XNode } from '@/api/xnode'
import { useDurableCommand } from '@/composables/useDurableCommand'
import OperationStatusNotice from '@/components/common/OperationStatusNotice.vue'
import { useI18n } from '@/i18n'
const props = defineProps<{ node: XNode | null }>()
const emit = defineEmits<{ close: []; removed: [] }>()
const { t } = useI18n()
const command = useDurableCommand({ errorKey: 'xnode.saveFailed', onTerminal: receipt => { if (receipt.status === 'succeeded') emit('removed') } })
const { receipt, error, checking, busy, blocksMutations, refresh } = command
const message = computed(() => receipt.value?.errorCode ? t(`nodeRemoval.blockers.${receipt.value.errorCode}`) : receipt.value?.status === 'succeeded' ? t('nodeRemoval.done') : null)
async function remove() {
 if (!props.node) return
 await command.execute(props.node.id, props.node.id, key => xnodeApi.removeNode(props.node!.id, key))
}
function close() { if (!busy.value) { command.reset(); emit('close') } }
</script>
<template>
  <UModal :open="!!node" :title="t('nodeRemoval.title', { name: node?.name ?? '' })" :description="t('nodeRemoval.description')" :dismissible="!busy" @update:open="!$event && close()">
    <template #body><OperationStatusNotice :receipt="receipt" :error="error" :checking="checking" :message="message" @refresh="refresh" /></template>
    <template #footer><div class="flex w-full justify-end gap-2"><UButton color="neutral" variant="ghost" :disabled="busy" :label="t('common.close')" @click="close" /><UButton v-if="receipt?.status !== 'succeeded'" color="error" :loading="busy" :disabled="blocksMutations" :label="t('nodeRemoval.remove')" @click="remove" /></div></template>
  </UModal>
</template>
