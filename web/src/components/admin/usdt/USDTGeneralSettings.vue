<script setup lang="ts">
import type { USDTConfig } from '@/api/usdt'
import SwitchField from '@/components/common/SwitchField.vue'
import { useI18n } from '@/i18n'
const model = defineModel<USDTConfig>({ required: true })
const { t } = useI18n()
function set<K extends keyof USDTConfig>(key: K, value: USDTConfig[K]): void { model.value = { ...model.value, [key]: value } }
</script>

<template>
  <div class="usdt-general">
    <SwitchField id="usdt-enabled" :model-value="model.enabled" :label="t('common.enabled')" :help="t('adminUsdt.quoteHint')" @update:model-value="set('enabled', $event)" />
    <div class="usdt-form-grid">
      <UFormField name="name" :label="t('adminUsdt.name')"><UInput :model-value="model.name" @update:model-value="set('name', String($event))" /></UFormField>
      <UFormField name="rate" :label="t('adminUsdt.rate')" :description="t('adminUsdt.rateHint')"><UInput :model-value="model.rate" inputmode="decimal" @update:model-value="set('rate', String($event))" /></UFormField>
      <UFormField name="expiryMinutes" :label="t('adminUsdt.expiry')"><UInputNumber :model-value="model.expiryMinutes" :min="5" :max="120" @update:model-value="set('expiryMinutes', Number($event))" /></UFormField>
    </div>
  </div>
</template>
