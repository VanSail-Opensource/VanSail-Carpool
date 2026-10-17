<script setup lang="ts">
import { computed } from 'vue'
import type { XNodeSettings } from '@/api/xnode'
import SwitchField from '@/components/common/SwitchField.vue'
import { useI18n } from '@/i18n'

const model = defineModel<XNodeSettings>({ required: true })
const { t } = useI18n()
const enabled = computed({ get: () => !!model.value.bandwidth, set: value => { model.value = { ...model.value, bandwidth: value ? { normalMbps: 100, burstMbps: 0, afterSeconds: 0, resetSeconds: 0 } : null } } })
</script>

<template>
  <div class="grid gap-4 py-3">
    <SwitchField id="node-bandwidth-enabled" v-model="enabled" :label="t('nodeForms.bandwidth')" :help="t('nodeForms.bandwidthHint')" />
    <div v-if="model.bandwidth" class="grid gap-4 sm:grid-cols-2">
      <UFormField :label="t('nodeForms.normalMbps')"><UInput v-model.number="model.bandwidth.normalMbps" type="number" min="1" max="1000000" step="1" required /></UFormField>
      <UFormField :label="t('nodeForms.burstMbps')"><UInput v-model.number="model.bandwidth.burstMbps" type="number" min="0" max="1000000" step="1" required /></UFormField>
      <UFormField :label="t('nodeForms.afterSeconds')"><UInput v-model.number="model.bandwidth.afterSeconds" type="number" min="0" max="3600" step="1" required /></UFormField>
      <UFormField :label="t('nodeForms.resetSeconds')"><UInput v-model.number="model.bandwidth.resetSeconds" type="number" min="0" max="3600" step="1" required /></UFormField>
    </div>
  </div>
</template>
