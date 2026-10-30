<script setup lang="ts">
import { computed } from 'vue'
import type { FirewallFilter } from '@/api/firewall'
import { useI18n } from '@/i18n'
import FirewallFilterCondition from './FirewallFilterCondition.vue'

const props = defineProps<{ modelValue: FirewallFilter; depth: number; nodes: number; conditions: number }>()
const emit = defineEmits<{ 'update:modelValue': [value: FirewallFilter] }>()
const { t } = useI18n()
const options = computed(() => [{ value: 'and', label: t('firewall.matchAll') }, { value: 'or', label: t('firewall.matchAny') }])
function update(index: number, value: FirewallFilter) {
  const children = [...(props.modelValue.children ?? [])]
  children[index] = value
  emit('update:modelValue', { ...props.modelValue, children })
}
function remove(index: number) { emit('update:modelValue', { ...props.modelValue, children: (props.modelValue.children ?? []).filter((_, i) => i !== index) }) }
function add(group: boolean) {
  const condition: FirewallFilter = { field: 'rule', value: 'xray_tcp_probe' }
  const child: FirewallFilter = group ? { op: 'or', children: [condition] } : condition
  emit('update:modelValue', { ...props.modelValue, children: [...(props.modelValue.children ?? []), child] })
}
</script>

<template>
  <div class="filter-group grid min-w-0 gap-2" :class="{ 'filter-group--nested': depth > 2 }">
    <div class="flex flex-wrap items-center gap-2">
      <USelect :model-value="modelValue.op" :items="options" :aria-label="t('firewall.matchMode')" class="w-40" @update:model-value="emit('update:modelValue', { ...modelValue, op: $event as 'and' | 'or' })" />
      <UButton variant="ghost" color="neutral" icon="i-ph-plus" :label="t('firewall.addCondition')" :disabled="conditions >= 32 || nodes >= 64" @click="add(false)" />
      <UButton v-if="depth < 3" variant="ghost" color="neutral" icon="i-ph-brackets-curly" :label="t('firewall.addGroup')" :disabled="conditions >= 32 || nodes > 62" @click="add(true)" />
    </div>
    <div v-for="(child, index) in modelValue.children ?? []" :key="index" class="flex min-w-0 items-start gap-2">
      <FirewallFilterGroup v-if="child.op" :model-value="child" :depth="depth + 1" :nodes="nodes" :conditions="conditions" class="flex-1" @update:model-value="update(index, $event)" />
      <FirewallFilterCondition v-else :model-value="child" @update:model-value="update(index, $event)" />
      <UButton icon="i-ph-trash" variant="ghost" color="neutral" :aria-label="t('firewall.removeFilter')" @click="remove(index)" />
    </div>
  </div>
</template>

<style scoped>
.filter-group--nested { border-inline-start: 2px solid var(--ui-border-muted); padding-inline-start: .75rem; }
</style>
