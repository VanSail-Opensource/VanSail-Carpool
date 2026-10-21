<script setup lang="ts">
import { computed, shallowRef } from 'vue'
import { useI18n } from '@/i18n'
import MemberRoutingEditor from '@/components/subscription/MemberRoutingEditor.vue'
import OperationStatusNotice from '@/components/common/OperationStatusNotice.vue'
import DIYSettingsGroup from './DIYSettingsGroup.vue'
import DIYCompatibility from './DIYCompatibility.vue'
import { useDIY } from './useDIY'

const { t } = useI18n()
const state = useDIY()
const advanced = shallowRef(false)
const basic = computed(() => state.view.value?.fields.filter(field => !field.advanced) ?? [])
const groups = computed(() => [...new Set(state.view.value?.fields.filter(field => field.advanced).map(field => field.group) ?? [])])
const routePending = computed(() => state.view.value && !['active', 'disabled'].includes(state.view.value.routing.plan.phase))
const routingError = computed(() => state.errors.value.routing ? t(`managedRouting.errors.${state.errors.value.routing}`) : '')
</script>

<template>
  <div class="page diy-center">
    <header class="flex justify-between items-start gap-3"><div><h1>{{ t('diy.title') }}</h1><p class="text-sm text-muted">{{ t('diy.description') }}</p></div><UButton icon="i-ph-arrow-clockwise" variant="ghost" color="neutral" :aria-label="t('common.refresh')" :loading="state.loading.value" :disabled="state.submitting.value" @click="state.load(true)" /></header>
    <UProgress v-if="state.loading.value" />
    <UAlert v-if="state.error.value" color="warning" :title="state.error.value" />
    <UButton v-if="state.error.value && !state.view.value" class="justify-self-start" variant="outline" :label="t('common.tryAgain')" @click="state.load(true)" />
    <template v-if="state.view.value">
      <section class="grid gap-4">
        <div class="flex justify-between items-center gap-3"><h2>{{ t('diy.subscription') }}</h2><UButton variant="ghost" color="neutral" icon="i-ph-arrow-u-up-left" :label="t('diy.reset')" :disabled="state.submitting.value" @click="state.reset" /></div>
        <p class="text-xs text-muted">{{ t('diy.resetHint') }}</p>
        <DIYSettingsGroup v-model="state.overrides.value" :fields="basic" :errors="state.errors.value" :disabled="state.submitting.value" />
        <details class="diy-advanced" @toggle="advanced = ($event.target as globalThis.HTMLDetailsElement).open">
          <summary>{{ t('diy.advanced') }}</summary>
          <div v-if="advanced" class="grid gap-6 py-3"><DIYSettingsGroup v-for="group in groups" :key="group" v-model="state.overrides.value" :group="group" :fields="state.view.value.fields.filter(field => field.group === group && field.advanced)" :errors="state.errors.value" :disabled="state.submitting.value" /></div>
        </details>
        <UAlert v-if="state.errors.value.overrides" color="warning" :title="t(`diy.errors.${state.errors.value.overrides}`)" />
        <UProgress v-if="state.autoPreview.pending.value" />
        <UAlert v-if="state.autoPreview.error.value" color="warning" :title="state.autoPreview.error.value" />
        <DIYCompatibility :items="state.compatibility.value" />
      </section>
      <section class="border-t border-default pt-4"><MemberRoutingEditor external :initial="state.view.value.routing" :disabled="state.submitting.value" @draft="state.routeDraft" @refresh="state.load(true)" @save="state.save" /><UAlert v-if="routingError" color="warning" :title="routingError" /></section>
      <OperationStatusNotice :receipt="state.tracker.receipt.value" :error="state.tracker.error.value" :checking="state.tracker.checking.value" @refresh="state.tracker.refresh" />
      <p v-if="state.saved.value" role="status" class="text-sm text-success">{{ t(routePending ? 'diy.routingPending' : 'diy.saved') }}</p>
      <div class="diy-save"><UButton :label="t('common.save')" :loading="state.submitting.value" :disabled="state.submitting.value || state.loading.value" @click="state.save" /></div>
    </template>
  </div>
</template>

<style scoped>
.diy-center { display: grid; gap: 1rem; max-width: 900px; margin-inline: auto; padding-bottom: 2rem; }
.diy-center h1 { font-size: 1.3rem; margin: 0 0 .4rem; }
.diy-center h2 { font-size: 1rem; margin: 0; }
.diy-center p { margin: 0; }
.diy-advanced > summary { cursor: pointer; padding-block: .65rem; font-size: .85rem; }
.diy-save { position: sticky; bottom: 0; display: flex; justify-content: flex-end; padding-block: .65rem; background: var(--canvas); }
@media(max-width:599px) { .diy-save { bottom: calc(var(--shell-safe-bottom, 0px)); } }
</style>
