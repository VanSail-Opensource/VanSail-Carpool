<script setup lang="ts">
import { computed, shallowRef } from 'vue'
import JsonDocumentField from '@/components/common/JsonDocumentField.vue'
import { useI18n } from '@/i18n'
import { useMemberRouting } from './useMemberRouting'

const { t } = useI18n()
const state = useMemberRouting()
const validOutbounds = shallowRef(true)
const validRules = shallowRef(true)
const selection = shallowRef<number>()
const options = computed(() => state.templates.value.map((item, value) => ({ value, label: item.name })))
</script>

<template>
  <section class="section-block grid gap-4 min-w-0">
    <div class="section-heading"><h2>{{ t('memberNodes.routing') }}</h2><UButton variant="ghost" color="neutral" icon="i-ph-arrow-clockwise" :aria-label="t('common.refresh')" :disabled="state.busy.value" :loading="state.loading.value" @click="state.load" /></div>
    <p class="text-sm text-muted">{{ t('memberNodes.routingHint') }}</p>
    <UAlert v-if="state.error.value" color="warning" :title="state.error.value" />
    <p v-if="state.saved.value" role="status" class="text-sm text-success">{{ t('memberNodes.saved') }}</p>
    <UProgress v-if="state.loading.value" />
    <p v-else-if="!state.nodes.value.length && !state.error.value" class="text-muted">{{ t('memberNodes.noNodes') }}</p>
    <template v-if="state.nodes.value.length">
      <UFormField :label="t('xnode.node')"><USelect v-model="state.node.value" :items="state.nodes.value" :disabled="state.busy.value" class="w-full" /></UFormField>
      <template v-if="state.current.value && !state.loading.value">
        <div class="flex flex-wrap items-end gap-2"><UFormField :label="t('memberNodes.template')" class="grow"><USelect v-model="selection" :items="options" class="w-full" /></UFormField><UButton variant="outline" color="neutral" :label="t('memberNodes.insert')" :disabled="selection === undefined || state.busy.value" @click="selection !== undefined && state.insert(selection)" /></div>
        <JsonDocumentField :key="`${state.node.value}-outbounds`" v-model="state.outbounds.value" :disabled="state.busy.value" :label="t('memberNodes.outbounds')" :rows="10" @valid="validOutbounds = $event" />
        <UButton class="justify-self-end" :label="t('memberNodes.saveOutbounds')" :loading="state.busy.value" :disabled="!validOutbounds" @click="state.save('outbounds')" />
        <JsonDocumentField :key="`${state.node.value}-rules`" v-model="state.rules.value" :disabled="state.busy.value" :label="t('memberNodes.rules')" :rows="10" @valid="validRules = $event" />
        <UButton class="justify-self-end" :label="t('memberNodes.saveRules')" :loading="state.busy.value" :disabled="!validRules" @click="state.save('rules')" />
      </template>
    </template>
  </section>
</template>
