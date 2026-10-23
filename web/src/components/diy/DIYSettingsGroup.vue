<script setup lang="ts">
import type { DIYField, DIYOverrides } from '@/api/xnodeMember'
import { useI18n } from '@/i18n'
import DIYFieldControl from './DIYFieldControl.vue'
const model = defineModel<DIYOverrides>({ required: true })
defineProps<{ fields: DIYField[]; disabled?: boolean; errors: Record<string, string> }>()
const { t } = useI18n()
function set(key: string, value: unknown) {
  const next = { ...model.value } as Record<string, unknown>
  if (value === undefined) delete next[key]
  else next[key] = value
  model.value = next as DIYOverrides
}
const value = (key: string) => (model.value as Record<string, unknown>)[key]
</script>
<template>
  <section class="grid gap-3 min-w-0">
    <div class="grid gap-3 sm:grid-cols-2">
      <DIYFieldControl v-for="field in fields" :key="field.key" :field="field" :model-value="value(field.key)" :disabled="disabled" :error="errors[field.key] ? t(`diy.errors.${errors[field.key]}`) : undefined" :class="field.kind === 'headers' ? 'sm:col-span-2' : ''" @update:model-value="set(field.key, $event)" />
    </div>
  </section>
</template>
