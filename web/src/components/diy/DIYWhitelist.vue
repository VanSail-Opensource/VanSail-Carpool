<script setup lang="ts">
import { computed, shallowRef } from 'vue'
import { z } from 'zod'
import type { DIYWhitelist, DIYWhitelistView } from '@/api/xnodeMember'
import { useI18n } from '@/i18n'
import { selectionHaptic } from '@/utils/telegramHaptics'

const model = defineModel<DIYWhitelist>({ required: true })
const props = defineProps<{ view?: DIYWhitelistView; disabled?: boolean; error?: string }>()
const { t } = useI18n()
const ip = shallowRef('')
const schema = z.union([z.ipv4(), z.ipv6()])
const quota = computed(() => props.view?.quota ?? 5)
function comparable(value: string) { try { return new globalThis.URL(value.includes(':') ? `http://[${value}]` : `http://${value}`).hostname } catch { return value } }
const dirty = computed(() => props.view && (model.value.enabled !== props.view.enabled || JSON.stringify(model.value.ips.map(comparable)) !== JSON.stringify(props.view.ips.map(comparable))))
const valid = computed(() => schema.safeParse(ip.value.trim()).success && !model.value.ips.includes(ip.value.trim()) && model.value.ips.length < quota.value)
function add() { if (!valid.value) return; model.value = { ...model.value, ips: [...model.value.ips, ip.value.trim()] }; ip.value = '' }
function remove(value: string) { model.value = { ...model.value, ips: model.value.ips.filter(item => item !== value) } }
</script>

<template>
  <section class="grid gap-3">
    <div class="flex justify-between items-center gap-3"><span class="text-xs text-muted">{{ t('diy.whitelist.quota', { used: model.ips.length, quota }) }}</span></div>
    <USwitch :model-value="model.enabled" :disabled="disabled || quota === 0" :label="t('diy.whitelist.enabled')" :description="t('diy.whitelist.hint')" @update:model-value="selectionHaptic(); model = { ...model, enabled: $event }" />
    <p v-if="view" class="text-xs text-muted" role="status">{{ t(dirty ? 'diy.whitelist.unsaved' : `diy.whitelist.phase.${view.phase}`, { count: view.pendingNodes.length }) }}</p>
    <UAlert v-if="error" color="warning" :title="error" />
    <UFormField :label="t('firewall.ip')">
      <div class="flex gap-2"><UInput v-model="ip" class="min-w-0 flex-1 font-mono" autocomplete="off" :disabled="disabled || model.ips.length >= quota" @keydown.enter.prevent="add" /><UButton data-haptic="action" icon="i-ph-plus" variant="outline" color="neutral" :aria-label="t('diy.whitelist.add')" :disabled="disabled || !valid" @click="add" /></div>
    </UFormField>
    <p v-if="model.ips.length === 0" class="text-xs text-muted">{{ t(model.enabled ? 'diy.whitelist.blocksAll' : 'diy.whitelist.empty') }}</p>
    <ul v-else class="grid gap-1"><li v-for="value in model.ips" :key="value" class="flex items-center justify-between gap-2 py-1"><code class="text-sm break-all">{{ value }}</code><UButton data-haptic="action" icon="i-ph-trash" variant="ghost" color="neutral" :aria-label="t('common.delete')" :disabled="disabled || quota === 0" @click="remove(value)" /></li></ul>
  </section>
</template>
