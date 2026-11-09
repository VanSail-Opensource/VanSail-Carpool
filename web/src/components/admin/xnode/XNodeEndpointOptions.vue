<script setup lang="ts">
import { computed, shallowRef, watch } from 'vue'
import type { XNodeEndpointField } from '@/api/xnode'
import type { DIYField } from '@/api/xnodeMember'
import DIYFieldControl from '@/components/diy/DIYFieldControl.vue'
import { useI18n } from '@/i18n'
import { endpointOptionActive } from './endpointForm'
import { endpointError, endpointFieldLabel } from './endpointLabels'
import XNodeEndpointPortRanges from './XNodeEndpointPortRanges.vue'
const model = defineModel<Record<string, unknown>>({ required: true })
const props = defineProps<{ fields: XNodeEndpointField[]; errors: Record<string, string>; disabled?: boolean }>()
const { t } = useI18n()
const selected = shallowRef('')
const fields = computed(() => props.fields.filter(field => endpointOptionActive(field, model.value)))
const groups = computed(() => [...new Set(fields.value.map(field => field.group))].map(value => {
  const group = t(`endpointOptions.groups.${value}`), count = fields.value.filter(field => field.group === value && props.errors[field.key]).length
  return { value, label: count ? t('endpointOptions.groupErrors', { group, count }) : group }
}))
watch(groups, items => { if (!items.some(item => item.value === selected.value)) selected.value = items[0]?.value ?? '' }, { immediate: true })
const visible = computed(() => fields.value.filter(field => field.group === selected.value))
function update(key: string, value: unknown) {
  const next = { ...model.value }
  if (value === undefined) delete next[key]
  else next[key] = value
  model.value = next
}
const scalar = (field: XNodeEndpointField) => field as DIYField
const error = (key: string) => props.errors[key] ? endpointError(props.errors[key]) : undefined
</script>
<template>
  <section class="grid min-w-0 gap-3">
    <div><h4 class="text-sm font-medium">{{ t('nodeForms.endpointOptions') }}</h4><p class="text-xs text-muted">{{ t('endpointOptions.inheritHint') }}</p></div>
    <UAlert v-if="Object.keys(errors).length" color="warning" :title="t('endpointOptions.review')" />
    <p v-if="!fields.length" class="text-sm text-muted">{{ t('endpointOptions.noOptions') }}</p>
    <template v-else>
      <div class="min-w-0 overflow-x-auto"><UTabs v-model="selected" :items="groups" :content="false" variant="link" :ui="{ root: 'items-start', list: 'w-max min-w-full' }" /></div>
      <div class="grid min-w-0 gap-3 sm:grid-cols-2">
        <template v-for="field in visible" :key="field.key">
          <XNodeEndpointPortRanges v-if="field.kind === 'port-ranges'" :model-value="model[field.key]" :label="endpointFieldLabel(field.key)" :error="error(field.key)" :disabled="disabled" @update:model-value="update(field.key, $event)" />
          <DIYFieldControl v-else :field="scalar(field)" :label="endpointFieldLabel(field.key)" :model-value="model[field.key]" :error="error(field.key)" :disabled="disabled" resettable :class="field.kind === 'headers' ? 'sm:col-span-2' : ''" @update:model-value="update(field.key, $event)" />
        </template>
      </div>
    </template>
  </section>
</template>
