<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from '@/i18n'
import MemberRoutingGraph from './MemberRoutingGraph.vue'
import { useMemberRouting } from './useMemberRouting'

const { t } = useI18n()
const state = useMemberRouting()
const filtered = computed(() => state.choices.value.map((rule, index) => ({ rule, index })).filter(({ rule }) => !state.filterNode.value || rule.inboundNodeId === state.filterNode.value || rule.outboundNodeId === state.filterNode.value))
const labels = computed(() => Object.fromEntries(state.choices.value.map(rule => [rule.id, state.current.value?.templates.find(item => item.id === rule.templateId)?.name ?? rule.templateId])))
</script>

<template>
  <section class="section-block grid gap-4 min-w-0">
    <div class="flex items-center justify-between gap-3"><h2>{{ t('memberNodes.routing') }}</h2><UButton variant="ghost" color="neutral" icon="i-ph-arrow-clockwise" :aria-label="t('common.refresh')" :disabled="state.locked.value" :loading="state.loading.value" @click="state.load" /></div>
    <p class="text-sm text-muted">{{ t('managedRouting.description') }}</p>
    <UAlert v-if="state.error.value || state.command.error.value" color="warning" :title="state.error.value || state.command.error.value || ''" />
    <UButton v-if="state.command.error.value && state.command.receipt.value" class="justify-self-start" variant="outline" :label="t('common.tryAgain')" :loading="state.command.checking.value" @click="state.command.refresh" />
    <UProgress v-if="state.loading.value" />
    <UButton v-if="state.error.value" variant="outline" :label="t('common.tryAgain')" @click="state.load" />
    <template v-else-if="state.current.value">
      <UAlert v-if="state.current.value.plan.reason" color="warning" :title="t('managedRouting.disabled')" />
      <p v-if="!state.nodes.value.length" class="text-muted">{{ t('memberNodes.noNodes') }}</p>
      <template v-else>
        <div class="grid gap-3 sm:grid-cols-2"><UFormField :label="t('managedRouting.previewFrom')"><USelect v-model="state.start.value" :items="state.nodes.value" class="w-full" /></UFormField><UFormField :label="t('managedRouting.destination')"><UInput v-model="state.destination.value" :placeholder="t('managedRouting.destinationHint')" class="w-full" /></UFormField></div>
        <MemberRoutingGraph :nodes="state.current.value.nodes" :paths="state.paths.value" :trace="state.trace.value" :selected="state.filterNode.value" :labels="labels" @select="state.filterNode.value = state.filterNode.value === $event ? '' : $event" />
        <UButton v-if="state.filterNode.value" class="justify-self-start" color="neutral" variant="ghost" :label="t('managedRouting.showAll')" @click="state.filterNode.value = ''" />
        <form class="grid gap-4" @submit.prevent="state.save">
          <fieldset v-for="{ rule, index } in filtered" :key="rule.id" :disabled="state.locked.value" class="grid gap-3 min-w-0 border-b border-default pb-4">
            <legend class="text-sm text-muted mb-2">{{ t('managedRouting.ruleNumber', { number: index + 1 }) }}</legend>
            <div class="grid gap-3 md:grid-cols-3">
              <UFormField :label="t('managedRouting.inbound')"><USelect v-model="rule.inboundNodeId" :items="state.nodes.value.filter(n => n.value !== rule.outboundNodeId)" class="w-full" /></UFormField>
              <UFormField :label="t('memberNodes.template')"><USelect v-model="rule.templateId" :items="state.templates.value" class="w-full" /></UFormField>
              <UFormField :label="t('managedRouting.outbound')"><USelect v-model="rule.outboundNodeId" :items="state.nodes.value.filter(n => n.value !== rule.inboundNodeId)" class="w-full" /></UFormField>
            </div>
            <div class="flex justify-end gap-1"><UButton type="button" variant="ghost" color="neutral" icon="i-ph-arrow-up" :aria-label="t('managedRouting.moveUp')" :disabled="index === 0" @click="state.move(index, -1)" /><UButton type="button" variant="ghost" color="neutral" icon="i-ph-arrow-down" :aria-label="t('managedRouting.moveDown')" :disabled="index === state.choices.value.length - 1" @click="state.move(index, 1)" /><UButton type="button" variant="ghost" color="neutral" icon="i-ph-trash-bold" :aria-label="t('common.delete')" @click="state.choices.value.splice(index, 1)" /></div>
          </fieldset>
          <p v-if="!state.choices.value.length" class="text-sm text-muted">{{ t('managedRouting.empty') }}</p>
          <p v-if="!state.templates.value.length" class="text-sm text-muted">{{ t('managedRouting.noTemplates') }}</p>
          <UAlert v-if="state.previewError.value" color="warning" :title="state.previewError.value" />
          <div class="flex justify-between gap-2"><UButton type="button" variant="outline" color="neutral" icon="i-ph-plus-bold" :label="t('managedRouting.add')" :disabled="state.locked.value || state.nodes.value.length < 2 || !state.templates.value.length || state.choices.value.length >= 64" @click="state.add" /><UButton type="submit" :loading="state.command.submitting.value || state.previewing.value" :disabled="!state.canSave.value" :label="t('common.save')" /></div>
          <p v-if="state.locked.value" role="status" class="text-sm text-muted">{{ t('managedRouting.deploying') }}</p>
        </form>
      </template>
    </template>
  </section>
</template>
