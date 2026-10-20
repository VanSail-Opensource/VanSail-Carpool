<script setup lang="ts">
import { nextTick, shallowRef, useTemplateRef, watch } from 'vue'
import { useI18n } from '@/i18n'
import { useXNodeLogs } from './useXNodeLogs'

const open = defineModel<boolean>('open', { required: true })
const props = defineProps<{ nodeId: string; name: string }>()
const { t } = useI18n()
const state = useXNodeLogs(() => open.value ? props.nodeId : '')
const viewport = useTemplateRef<InstanceType<typeof globalThis.HTMLElement>>('viewport')
const following = shallowRef(true)
watch(state.text, () => { if (following.value) void nextTick(() => { if (viewport.value) viewport.value.scrollTop = viewport.value.scrollHeight }) })
function scroll() { const element = viewport.value; if (element) following.value = element.scrollHeight - element.scrollTop - element.clientHeight < 48 }
</script>

<template>
  <UModal v-model:open="open" :title="t('xnode.liveLogs', { name })" :description="t('xnode.logsHint')" :close="{ 'aria-label': t('common.close') }" :ui="{ content: 'max-w-5xl' }" scrollable>
    <template #body>
      <div class="grid gap-3 min-w-0">
        <div class="flex items-center justify-between gap-3 flex-wrap">
          <UBadge v-if="state.snapshot.value" :color="state.snapshot.value.stale ? 'warning' : 'success'" variant="subtle">{{ t(state.snapshot.value.stale ? 'xnode.stale' : 'xnode.online') }}</UBadge>
          <div class="flex items-center gap-2"><USwitch v-model="state.paused.value" :label="t('xnode.pauseLogs')" /><UButton variant="ghost" color="neutral" icon="i-ph-arrow-clockwise" :aria-label="t('common.refresh')" :loading="state.loading.value" @click="state.refresh" /></div>
        </div>
        <UProgress v-if="state.loading.value && !state.snapshot.value" />
        <UAlert v-if="state.error.value" color="warning" :title="state.error.value" />
        <p v-if="state.snapshot.value?.dropped !== '0' && state.snapshot.value" class="text-xs text-muted">{{ t('xnode.droppedLogs', { count: state.snapshot.value.dropped }) }}</p>
        <pre v-if="state.text.value" ref="viewport" class="node-logs" tabindex="0" :aria-label="t('xnode.logs')" @scroll.passive="scroll">{{ state.text.value }}</pre>
        <p v-else-if="!state.loading.value && !state.error.value" class="text-muted">{{ t('xnode.noLogs') }}</p>
      </div>
    </template>
  </UModal>
</template>

<style scoped>
.node-logs { max-height: min(60dvh, 580px); min-height: 160px; overflow: auto; white-space: pre-wrap; overflow-wrap: anywhere; padding: 12px; margin: 0; background: var(--ui-bg-elevated); border-radius: 8px; font-size: 12px; line-height: 1.6; }
</style>
