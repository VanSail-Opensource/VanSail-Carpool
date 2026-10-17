<script setup lang="ts">
import { computed } from 'vue'
import type { XNodeEndpoint } from '@/api/xnode'
import SwitchField from '@/components/common/SwitchField.vue'
import { useI18n } from '@/i18n'
import { objectOption } from './nodeFormOptions'

const model = defineModel<XNodeEndpoint['overrides']>({ required: true })
const { t } = useI18n()
const securityFields = ['serverName', 'fingerprint', 'alpn', 'publicKey', 'shortId'] as const
const transportFields = ['path', 'host', 'serviceName', 'authority', 'mode'] as const
function text(group: string, key: string) { const value = objectOption(model.value?.[group])[key]; return Array.isArray(value) ? value.join(', ') : typeof value === 'string' ? value : '' }
function update(group: string, key: string, value: string | boolean) {
  const next = { ...objectOption(model.value?.[group]) }
  if (value === '') delete next[key]
  else next[key] = key === 'alpn' ? String(value).split(/[\s,]+/).filter(Boolean) : value
  model.value = { ...model.value, [group]: next }
}
const insecure = computed({ get: () => objectOption(model.value?.securityOptions).allowInsecure === true, set: value => update('securityOptions', 'allowInsecure', value) })
const fromAddress = computed({ get: () => objectOption(model.value?.legacyHost).overrideSniFromAddress === true, set: value => update('legacyHost', 'overrideSniFromAddress', value) })
const blankSNI = computed({ get: () => objectOption(model.value?.legacyHost).keepSniBlank === true, set: value => update('legacyHost', 'keepSniBlank', value) })
</script>

<template>
  <details>
    <summary class="cursor-pointer text-sm font-medium">{{ t('nodeForms.endpointOptions') }}</summary>
    <div class="grid gap-4 pt-4 sm:grid-cols-2">
      <UFormField v-for="field in securityFields" :key="field" :label="t(`nodeForms.${field}`)"><UInput :model-value="text('securityOptions', field)" class="w-full" @update:model-value="update('securityOptions', field, String($event))" /></UFormField>
      <UFormField v-for="field in transportFields" :key="field" :label="t(`nodeForms.${field}`)"><UInput :model-value="text('transportOptions', field)" class="w-full" @update:model-value="update('transportOptions', field, String($event))" /></UFormField>
    </div>
    <SwitchField id="endpoint-insecure" v-model="insecure" :label="t('nodeForms.allowInsecure')" />
    <SwitchField id="endpoint-sni-address" v-model="fromAddress" :label="t('nodeForms.sniFromAddress')" />
    <SwitchField id="endpoint-sni-blank" v-model="blankSNI" :label="t('nodeForms.blankSni')" />
  </details>
</template>
