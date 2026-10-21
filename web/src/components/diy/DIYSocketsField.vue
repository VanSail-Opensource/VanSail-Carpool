<script setup lang="ts">
import { useI18n } from '@/i18n'
type Row = { system: '' | 'linux' | 'android' | 'windows' | 'darwin' | 'ios' | 'freebsd'; network: '' | 'tcp' | 'tcp4' | 'tcp6' | 'udp' | 'udp4' | 'udp6'; level: string; opt: string; value: string; type: 'int' | 'str' }
const props = defineProps<{ modelValue?: Row[]; disabled?: boolean }>()
const emit = defineEmits<{ 'update:modelValue': [value: Row[]] }>()
const { t } = useI18n()
const systems = ['', 'linux', 'android', 'windows', 'darwin', 'ios', 'freebsd']
const networks = ['', 'tcp', 'tcp4', 'tcp6', 'udp', 'udp4', 'udp6']
function update(index: number, key: keyof Row, value: unknown) { emit('update:modelValue', (props.modelValue ?? []).map((row, i) => i === index ? { ...row, [key]: String(value) } : row)) }
const options = (values: string[]) => values.map(value => ({ value: value || '_system', label: value || t('diy.system') }))
</script>
<template>
  <div class="grid gap-3">
    <div v-for="(row, index) in modelValue ?? []" :key="index" class="grid gap-2 border-b border-default pb-3 sm:grid-cols-3">
      <UFormField :label="t('diy.socketSystem')"><USelect :model-value="row.system || '_system'" :items="options(systems)" :disabled="disabled" class="w-full" @update:model-value="update(index, 'system', $event === '_system' ? '' : $event)" /></UFormField>
      <UFormField :label="t('diy.socketNetwork')"><USelect :model-value="row.network || '_system'" :items="options(networks)" :disabled="disabled" class="w-full" @update:model-value="update(index, 'network', $event === '_system' ? '' : $event)" /></UFormField>
      <UFormField :label="t('diy.socketType')"><USelect :model-value="row.type" :items="[{ value: 'int', label: 'int' }, { value: 'str', label: 'str', disabled: row.system === 'windows' }]" :disabled="disabled" class="w-full" @update:model-value="update(index, 'type', $event)" /></UFormField>
      <UFormField :label="t('diy.socketLevel')"><UInput :model-value="row.level" inputmode="numeric" :disabled="disabled" :maxlength="16" class="w-full" @update:model-value="update(index, 'level', $event)" /></UFormField>
      <UFormField :label="t('diy.socketOption')"><UInput :model-value="row.opt" inputmode="numeric" :disabled="disabled" :maxlength="16" class="w-full" @update:model-value="update(index, 'opt', $event)" /></UFormField>
      <UFormField :label="t('diy.socketValue')"><UInput :model-value="row.value" :disabled="disabled" :maxlength="512" class="w-full" @update:model-value="update(index, 'value', $event)" /></UFormField>
      <UButton class="justify-self-end sm:col-span-3" color="neutral" variant="ghost" icon="i-ph-trash" :label="t('common.delete')" :disabled="disabled" @click="emit('update:modelValue', modelValue!.filter((_, i) => i !== index))" />
    </div>
    <UButton class="justify-self-start" icon="i-ph-plus" color="neutral" variant="outline" :label="t('diy.addRow')" :disabled="disabled || (modelValue?.length ?? 0) >= 32" @click="emit('update:modelValue', [...(modelValue ?? []), { system: '', network: '', level: '', opt: '', value: '', type: 'int' }])" />
  </div>
</template>
