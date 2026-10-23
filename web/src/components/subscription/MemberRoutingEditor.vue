<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from '@/i18n'
import MemberRoutingGraph from './MemberRoutingGraph.vue'
import RouteSpeedtestPanel from './RouteSpeedtestPanel.vue'
import { useMemberRouting } from './useMemberRouting'
import type { RoutingChoice, RoutingView } from '@/api/xnodeMember'

const props = defineProps<{ external?: boolean; initial?: RoutingView; disabled?: boolean }>()
const emit = defineEmits<{ draft: [value: RoutingChoice[]]; refresh: []; save: [] }>()

const { t } = useI18n()
const state = useMemberRouting(props.external ? { initial: () => props.initial, onDraft: value => emit('draft', value), onRefresh: () => emit('refresh') } : undefined)
const filtered = computed(() => state.choices.value.map((rule, index) => ({ rule, index })).filter(({ rule }) => !state.filterNode.value || rule.inboundNodeId === state.filterNode.value || rule.outboundNodeId === state.filterNode.value))
const graphNodes = computed(() => { const participating = new Set(state.choices.value.flatMap(choice => [choice.inboundNodeId, choice.outboundNodeId])); return state.current.value?.nodes.filter(node => participating.has(node.id)) ?? [] })
</script>

<template>
  <section class="section-block grid gap-3 min-w-0">
    <div v-if="!external" class="flex items-center justify-between gap-3"><h2>{{ t(external ? 'diy.routing' : 'memberNodes.routing') }}</h2><UButton :data-haptic="external ? 'action' : undefined" variant="ghost" color="neutral" icon="i-ph-arrow-clockwise" :aria-label="t('common.refresh')" :disabled="state.locked.value" :loading="state.loading.value" @click="external ? emit('refresh') : state.load()" /></div>
    <p v-if="!external" class="text-sm text-muted">{{ t('managedRouting.description') }}</p>
    <UAlert v-if="state.error.value || state.command.error.value" color="warning" :title="state.error.value || state.command.error.value || ''" />
    <UButton :data-haptic="external ? 'action' : undefined" v-if="state.command.error.value && state.command.receipt.value" class="justify-self-start" variant="outline" :label="t('common.tryAgain')" :loading="state.command.checking.value" @click="state.command.refresh" />
    <UProgress v-if="state.loading.value" />
    <UButton :data-haptic="external ? 'action' : undefined" v-if="state.error.value" variant="outline" :label="t('common.tryAgain')" @click="state.load" />
    <template v-else-if="state.current.value">
      <UAlert v-if="state.current.value.plan.reason" color="warning" :title="t('managedRouting.disabled')" />
      <p v-if="!state.nodes.value.length" class="text-muted">{{ t('memberNodes.noNodes') }}</p>
      <template v-else>
        <div v-if="state.traceNodes.value.length" class="grid grid-cols-1 gap-2 sm:grid-cols-3"><UFormField :label="t('managedRouting.previewFrom')"><USelect v-model="state.start.value" :items="state.traceNodes.value" class="w-full min-w-0" /></UFormField><UFormField :label="t('managedRouting.traceHost')"><USelect v-model="state.startEndpoint.value" :items="state.traceEndpoints.value" :ui="{ value: 'whitespace-normal break-words', itemLabel: 'whitespace-normal break-words' }" class="w-full min-w-0" /></UFormField><UFormField :label="t('managedRouting.destination')"><UInput v-model="state.destination.value" :placeholder="t('managedRouting.destinationHint')" class="w-full" /></UFormField></div>
        <MemberRoutingGraph v-if="graphNodes.length" :nodes="graphNodes" :paths="state.paths.value" :trace="state.trace.value" :selected="state.filterNode.value" :choices="state.choices.value" :endpoints="state.endpoints.value" :templates="state.current.value.templates" :status="state.routeStatus.value" @select="state.selectNode" />
        <RouteSpeedtestPanel v-if="state.traceNodes.value.length" :eligible="state.canSpeedtest.value" :nodes="state.current.value.nodes" :endpoints="state.endpoints.value" :request="{ revision: state.current.value.plan.revision, start: state.start.value, startEndpointId: state.startEndpoint.value, destination: { domain: state.destination.value, ip: state.destination.value, port: 443, network: 'tcp' } }" />
        <UButton :data-haptic="external ? 'action' : undefined" v-if="state.filterNode.value" class="justify-self-start" color="neutral" variant="ghost" :label="t('managedRouting.showAll')" @click="state.filterNode.value = ''" />
        <form class="grid gap-3" @submit.prevent="external ? emit('save') : state.save()">
          <fieldset v-for="{ rule, index } in filtered" :key="rule.id" :disabled="state.locked.value || disabled" class="grid gap-3 min-w-0 border-b border-default pb-4">
            <legend class="text-sm text-muted mb-2">{{ t('managedRouting.ruleNumber', { number: index + 1 }) }}</legend>
            <div class="grid grid-cols-1 gap-2 md:grid-cols-3">
              <UFormField :label="t('managedRouting.inboundHost')"><USelect :model-value="rule.inboundEndpointId" :items="state.endpointItems.value.filter(endpoint => endpoint.nodeId !== rule.outboundNodeId)" :ui="{ value: 'whitespace-normal break-words', itemLabel: 'whitespace-normal break-words' }" class="w-full" @update:model-value="state.setEndpoint(rule, 'inbound', $event)" /></UFormField>
              <UFormField :label="t('memberNodes.template')"><USelect v-model="rule.templateId" :items="state.templates.value" class="w-full" /></UFormField>
              <UFormField :label="t('managedRouting.outboundHost')"><USelect :model-value="rule.outboundEndpointId" :items="state.endpointItems.value.filter(endpoint => endpoint.nodeId !== rule.inboundNodeId)" :ui="{ value: 'whitespace-normal break-words', itemLabel: 'whitespace-normal break-words' }" class="w-full" @update:model-value="state.setEndpoint(rule, 'outbound', $event)" /></UFormField>
            </div>
            <div class="flex justify-end gap-1"><UButton :data-haptic="external ? 'action' : undefined" type="button" variant="ghost" color="neutral" icon="i-ph-arrow-up" :aria-label="t('managedRouting.moveUp')" :disabled="index === 0" @click="state.move(index, -1)" /><UButton :data-haptic="external ? 'action' : undefined" type="button" variant="ghost" color="neutral" icon="i-ph-arrow-down" :aria-label="t('managedRouting.moveDown')" :disabled="index === state.choices.value.length - 1" @click="state.move(index, 1)" /><UButton :data-haptic="external ? 'action' : undefined" type="button" variant="ghost" color="neutral" icon="i-ph-trash-bold" :aria-label="t('common.delete')" @click="state.choices.value.splice(index, 1)" /></div>
          </fieldset>
          <p v-if="!state.choices.value.length" class="text-sm text-muted">{{ t('managedRouting.empty') }}</p>
          <p v-if="!state.templates.value.length" class="text-sm text-muted">{{ t('managedRouting.noTemplates') }}</p>
          <UAlert v-if="state.previewError.value" color="warning" :title="state.previewError.value" />
          <div class="flex justify-between gap-2"><UButton :data-haptic="external ? 'action' : undefined" type="button" variant="outline" color="neutral" icon="i-ph-plus-bold" :label="t('managedRouting.add')" :disabled="disabled || state.locked.value || state.nodes.value.length < 2 || !state.templates.value.length || state.choices.value.length >= 64" @click="state.add" /><UButton :data-haptic="external ? 'action' : undefined" v-if="!external" type="submit" :loading="state.command.submitting.value || state.previewing.value" :disabled="!state.canSave.value" :label="t('common.save')" /></div>
          <p v-if="state.locked.value" role="status" class="text-sm text-muted">{{ t('managedRouting.deploying') }}</p>
        </form>
      </template>
    </template>
  </section>
</template>
