<script setup lang="ts">
import { computed } from 'vue'
import type { USDTEndpoint, USDTNetworkID, USDTQuotaGroup } from '@/api/usdt'
import SwitchField from '@/components/common/SwitchField.vue'
import { useI18n } from '@/i18n'
import { supportsWS } from './configForm'
const model = defineModel<USDTEndpoint>({ required: true })
const props = defineProps<{ network: USDTNetworkID; groups: USDTQuotaGroup[]; index: number; last: boolean; prefix: string }>()
const emit = defineEmits<{ remove: []; move: [direction: number] }>()
const { t } = useI18n()
const groups = computed(() => props.groups.map(group => ({ label: group.id, value: group.id })))
function set<K extends keyof USDTEndpoint>(key: K, value: USDTEndpoint[K]): void { model.value = { ...model.value, [key]: value } }
const secretHint = (configured: boolean) => configured ? t('adminUsdt.keepSecret') : ''
</script>

<template>
  <fieldset class="usdt-endpoint">
    <legend>{{ t('adminUsdt.endpoint', { number: index + 1 }) }}</legend>
    <div class="usdt-toolbar">
      <code v-if="model.host">{{ model.host }}</code>
      <div class="button-row">
        <UButton color="neutral" variant="ghost" icon="i-ph-arrow-up" :disabled="index === 0" :aria-label="t('adminUsdt.moveUp')" @click="emit('move', -1)" />
        <UButton color="neutral" variant="ghost" icon="i-ph-arrow-down" :disabled="last" :aria-label="t('adminUsdt.moveDown')" @click="emit('move', 1)" />
        <UButton color="error" variant="ghost" icon="i-ph-trash" :aria-label="t('adminUsdt.removeEndpoint')" @click="emit('remove')" />
      </div>
    </div>
    <div class="usdt-form-grid">
      <UFormField :name="`${prefix}.group`" :label="t('adminUsdt.quotaGroup')"><USelect :model-value="model.group" :items="groups" @update:model-value="set('group', String($event))" /></UFormField>
      <UFormField :name="`${prefix}.rpcUrl`" :label="t('adminUsdt.rpcUrl')" :description="t(`adminUsdt.apiKinds.${network}`)"><UInput :model-value="model.rpcUrl" type="password" autocomplete="new-password" :placeholder="secretHint(model.rpcConfigured)" @update:model-value="set('rpcUrl', String($event))" /></UFormField>
      <UFormField v-if="network === 'aptos'" :name="`${prefix}.indexerUrl`" :label="t('adminUsdt.indexerUrl')"><UInput :model-value="model.indexerUrl" type="password" autocomplete="new-password" :placeholder="secretHint(model.indexerConfigured)" @update:model-value="set('indexerUrl', String($event))" /></UFormField>
      <UFormField :name="`${prefix}.apiKey`" :label="t('adminUsdt.apiKey')"><UInput :model-value="model.apiKey" type="password" autocomplete="new-password" :placeholder="secretHint(model.keyConfigured)" @update:model-value="set('apiKey', String($event))" /></UFormField>
    </div>
    <template v-if="supportsWS(network)">
      <SwitchField :id="`usdt-ws-${model.id}`" :model-value="model.useWs" :label="t('adminUsdt.useWS')" @update:model-value="set('useWs', $event)" />
      <UFormField v-if="model.useWs" :name="`${prefix}.wsUrl`" :label="t('adminUsdt.wsUrl')" :description="t('adminUsdt.wsHint')"><UInput :model-value="model.wsUrl" type="password" autocomplete="new-password" :placeholder="secretHint(model.wsConfigured)" @update:model-value="set('wsUrl', String($event))" /></UFormField>
    </template>
  </fieldset>
</template>
