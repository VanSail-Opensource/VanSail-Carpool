<script setup lang="ts">
import { computed, watch } from 'vue'
import type { XNode, XNodeConfiguration, XNodeGroup } from '@/api/xnode'
import { useI18n } from '@/i18n'
import { nodeInboundItems } from './nodeFormOptions'

const model = defineModel<XNodeGroup['bindings']>({ required: true })
const props = defineProps<{ nodes: XNode[]; configurations: XNodeConfiguration[]; disabled?: boolean }>()
const emit = defineEmits<{ valid: [value: boolean] }>()
const { t } = useI18n()
const nodes = computed(() => props.nodes.map(item => ({ value: item.id, label: item.name })))
const inbounds = (id: string) => nodeInboundItems(id, props.nodes, props.configurations)
const valid = computed(() => model.value.every(item => props.nodes.some(node => node.id === item.nodeId) && inbounds(item.nodeId).some(tag => tag.value === item.inboundTag)) && new Set(model.value.map(item => `${item.nodeId}:${item.inboundTag}`)).size === model.value.length)
watch(valid, value => emit('valid', value), { immediate: true })
function add() { if (props.disabled || !props.nodes.length) return; const nodeId = props.nodes[0]?.id ?? ''; model.value = [...model.value, { nodeId, inboundTag: inbounds(nodeId)[0]?.value ?? '' }] }
function setNode(index: number, nodeId: string) { if (props.disabled) return; model.value = model.value.map((item, position) => position === index ? { ...item, nodeId, inboundTag: inbounds(nodeId)[0]?.value ?? '' } : item) }
function setInbound(index: number, inboundTag: string) { if (props.disabled) return; model.value = model.value.map((item, position) => position === index ? { ...item, inboundTag } : item) }
function remove(index: number) { if (!props.disabled) model.value = model.value.filter((_, position) => position !== index) }
</script>

<template>
  <section class="grid gap-3">
    <UAlert v-if="!valid" color="warning" :title="t('nodeForms.bindingInvalid')" />
    <div class="flex items-center justify-between gap-3"><span class="text-sm font-medium">{{ t('nodeForms.binding') }}</span><UButton variant="ghost" color="neutral" icon="i-ph-plus" :disabled="disabled || !nodes.length" :aria-label="t('nodeForms.addBinding')" @click="add" /></div>
    <div v-for="(item, index) in model" :key="index" class="grid grid-cols-[minmax(0,1fr)_minmax(0,1fr)_auto] items-end gap-2">
      <UFormField :label="t('xnode.node')"><USelect :disabled="disabled" :model-value="item.nodeId" :items="nodes" class="w-full" @update:model-value="setNode(index, String($event))" /></UFormField>
      <UFormField :label="t('xnode.inboundTag')"><USelect :disabled="disabled" :model-value="item.inboundTag" :items="inbounds(item.nodeId)" class="w-full" @update:model-value="setInbound(index, String($event))" /></UFormField>
      <UButton :disabled="disabled" color="error" variant="ghost" icon="i-ph-trash" :aria-label="t('common.delete')" @click="remove(index)" />
    </div>
  </section>
</template>
