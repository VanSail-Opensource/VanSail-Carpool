<script setup lang="ts">
import { computed, shallowRef } from 'vue'
import { xnodeApi, type XNodeEndpoint, type XNodeCommand } from '@/api/xnode'
import { useDurableCommand } from '@/composables/useDurableCommand'
import OperationStatusNotice from '@/components/common/OperationStatusNotice.vue'
import { localizedError, useI18n } from '@/i18n'
import XNodeMetricDetails from './XNodeMetricDetails.vue'

const props = defineProps<{ nodeId: string; endpoints: XNodeEndpoint[] }>()
const { t } = useI18n()
const name = shallowRef<XNodeCommand['name']>('runtime.reload')
const jobId = shallowRef('')
const version = shallowRef('')
const force = shallowRef(false)
const peerEndpointId = shallowRef('')
const result = shallowRef<Record<string, unknown> | null>(null)
const resultError = shallowRef<string | null>(null)
const options = computed(() => ['runtime.reload', 'geodata.refresh', 'ddns.run', 'update.check', 'update.install', 'update.rollback', 'speedtest'].map(value => ({ value, label: t(`nodeAdmin.commands.${value}`) })))
const peers = computed(() => props.endpoints.filter(item => item.enabled && item.nodeId !== props.nodeId).map(item => ({ value: item.id, label: item.name })))
const command = useDurableCommand({ errorKey: 'xnode.saveFailed', onTerminal: async receipt => { if (receipt.status === 'succeeded') await loadResult(receipt.id) } })
const { receipt, error, checking, busy, blocksMutations, refresh } = command
const message = computed(() => receipt.value?.errorCode ? localizedError({ code: receipt.value.errorCode }, 'xnode.saveFailed') : null)
const invalid = computed(() => name.value === 'ddns.run' && !jobId.value.trim() || name.value === 'speedtest' && !peerEndpointId.value)
async function loadResult(id: string) {
  resultError.value = null
  try { result.value = (await xnodeApi.commandResult(id)).result }
  catch (caught) { resultError.value = localizedError(caught, 'xnode.loadFailed') }
}
async function run() {
  const body: XNodeCommand = { name: name.value, ...(name.value === 'runtime.reload' && force.value ? { force: true } : {}), ...(name.value === 'ddns.run' ? { jobId: jobId.value.trim() } : {}), ...(name.value === 'update.install' && version.value ? { version: version.value } : {}), ...(name.value === 'speedtest' ? { peerEndpointId: peerEndpointId.value } : {}) }
  result.value = null
  if (!invalid.value) await command.execute(`${props.nodeId}:${name.value}`, JSON.stringify(body), key => xnodeApi.command(props.nodeId, body, key))
}
</script>

<template>
  <form class="grid gap-4" @submit.prevent="run">
    <UFormField :label="t('nodeAdmin.command')"><USelect v-model="name" :items="options" :disabled="busy || blocksMutations" class="w-full" /></UFormField>
    <UCheckbox v-if="name === 'runtime.reload'" v-model="force" :label="t('nodeAdmin.forceReload')" :description="t('nodeAdmin.reloadHint')" :disabled="busy || blocksMutations" />
    <UFormField v-if="name === 'ddns.run'" :label="t('xnode.profileId')"><UInput v-model="jobId" required class="w-full" :disabled="busy || blocksMutations" /></UFormField>
    <UFormField v-if="name === 'update.install'" :label="t('xnode.updateVersion')"><UInput v-model="version" class="w-full" :disabled="busy || blocksMutations" /></UFormField>
    <UFormField v-if="name === 'speedtest'" :label="t('nodeAdmin.peer')"><USelect v-model="peerEndpointId" :items="peers" :disabled="busy || blocksMutations" class="w-full" /></UFormField>
    <UButton type="submit" :loading="busy" :disabled="!!invalid || blocksMutations" :label="t('nodeAdmin.run')" class="justify-self-end" />
    <OperationStatusNotice :receipt="receipt" :message="message" :error="error" :checking="checking" @refresh="refresh" />
    <div v-if="resultError" class="flex gap-2"><span>{{ resultError }}</span><UButton v-if="receipt" variant="ghost" :label="t('common.tryAgain')" @click="loadResult(receipt.id)" /></div>
    <XNodeMetricDetails v-if="result" :value="result" />
  </form>
</template>
