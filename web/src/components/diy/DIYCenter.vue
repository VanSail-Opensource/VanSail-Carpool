<script setup lang="ts">
import { computed, shallowRef } from 'vue'
import { selectionHaptic } from '@/utils/telegramHaptics'
import { useI18n } from '@/i18n'
import MemberRoutingEditor from '@/components/subscription/MemberRoutingEditor.vue'
import OperationStatusNotice from '@/components/common/OperationStatusNotice.vue'
import DIYTuningPanel from './DIYTuningPanel.vue'
import DIYCompatibility from './DIYCompatibility.vue'
import DIYSplitProfiles from './DIYSplitProfiles.vue'
import DIYWhitelist from './DIYWhitelist.vue'
import { useDIY } from './useDIY'

const { t } = useI18n()
const state = useDIY()
const tabs = computed(() => ['connection', 'xhttp', 'routing', 'whitelist', 'compatibility'].map(value => ({ value, label: t(`diy.tabs.${value}`) })))
const activeTab = shallowRef<string | number>('connection')
function selectTab(value: string | number) { if (value !== activeTab.value) selectionHaptic(); activeTab.value = value }
const routePending = computed(() => state.view.value && !['active', 'disabled'].includes(state.view.value.routing.plan.phase))
const routingError = computed(() => state.errors.value.routing ? t(`managedRouting.errors.${state.errors.value.routing}`) : '')
const cost = computed(() => {
  const quote = state.billing.value
  if (!quote) return ''
  if (!quote.changed) return t('diy.cost.unchanged')
  if (!quote.available) return t('diy.cost.unavailable')
  return quote.useQuota ? t('diy.cost.allowance', { remaining: quote.remaining }) : t('diy.cost.txb', { amount: quote.charge.display })
})
</script>

<template>
  <div class="page diy-center">
    <header class="flex justify-between items-start gap-3"><div><h1>{{ t('diy.title') }}</h1></div><UButton icon="i-ph-arrow-clockwise" variant="ghost" color="neutral" data-haptic="refresh" :aria-label="t('common.refresh')" :loading="state.loading.value" :disabled="state.submitting.value" @click="state.load(true)" /></header>
    <UProgress v-if="state.loading.value" />
    <UAlert v-if="state.error.value" color="warning" :title="state.error.value" />
    <UButton v-if="state.error.value && !state.view.value" class="justify-self-start" variant="outline" :label="t('common.tryAgain')" @click="state.load(true)" />
    <template v-if="state.view.value">
      <UTabs class="diy-tabs" :items="tabs" :model-value="activeTab" @update:model-value="selectTab" :unmount-on-hide="false" variant="link" color="neutral" size="sm" :ui="{ list: 'w-full overflow-x-auto', trigger: 'shrink-0 flex-none min-h-11 px-3', label: 'shrink-0 whitespace-nowrap text-xs sm:text-sm', content: 'pt-4 min-w-0' }">
        <template #content="{ item }">
          <div v-if="['connection', 'xhttp'].includes(item.value)" class="grid gap-4 min-w-0">
            <DIYSplitProfiles v-if="item.value === 'xhttp'" v-model="state.splitProfiles.value" :hosts="state.view.value.splitHosts ?? []" :disabled="state.submitting.value" :error="state.errors.value.splitProfiles ? t(`diy.errors.${state.errors.value.splitProfiles}`) : undefined" />
            <DIYTuningPanel v-model="state.overrides.value" :fields="state.view.value.fields" :groups="item.value === 'connection' ? ['tls', 'shadowsocks', 'mux', 'singboxMux', 'mihomoMux'] : ['xhttp', 'xmux']" :errors="state.errors.value" :disabled="state.submitting.value" />
            <UAlert v-if="state.errors.value.overrides" color="warning" :title="t(`diy.errors.${state.errors.value.overrides}`)" />
          </div>
          <div v-else-if="item.value === 'routing'"><MemberRoutingEditor external :initial="state.view.value.routing" :disabled="state.submitting.value" @draft="state.routeDraft" @refresh="state.load(true)" @save="state.save" /><UAlert v-if="routingError" color="warning" :title="routingError" /></div>
          <DIYCompatibility v-else-if="item.value === 'compatibility'" :items="state.compatibility.value" />
          <DIYWhitelist v-else v-model="state.whitelist.value" :view="state.view.value.whitelist" :disabled="state.submitting.value" :error="state.errors.value.whitelist ? t(`diy.errors.${state.errors.value.whitelist}`) : undefined" />
        </template>
      </UTabs>
      <UProgress v-if="state.autoPreview.pending.value" />
      <UAlert v-if="state.autoPreview.error.value" color="warning" :title="state.autoPreview.error.value" />
      <OperationStatusNotice :receipt="state.tracker.receipt.value" :error="state.tracker.error.value" :checking="state.tracker.checking.value" @refresh="state.tracker.refresh" />
      <Transition name="diy-feedback"><p v-if="state.saved.value" role="status" class="text-sm text-success">{{ t(routePending ? 'diy.routingPending' : state.view.value.whitelist?.phase === 'pending' ? 'diy.whitelist.savedPending' : 'diy.saved') }}</p></Transition>
      <div class="diy-save">
        <p role="status" class="text-xs text-muted">{{ cost }}</p>
        <div class="flex justify-end gap-2"><UButton data-haptic="action" variant="ghost" color="neutral" icon="i-ph-arrow-u-up-left" :label="t('diy.reset')" :disabled="state.submitting.value" @click="state.reset" /><UButton data-haptic="confirm" :label="t('common.save')" :loading="state.submitting.value" :disabled="state.submitting.value || state.loading.value || state.invalid.value || state.autoPreview.pending.value" @click="state.save" /></div>
      </div>
    </template>
  </div>
</template>

<style scoped>
.diy-center { display: grid; gap: 1rem; max-width: 900px; margin-inline: auto; padding-bottom: 2rem; min-width: 0; }
.diy-center h1 { font-size: 1.3rem; margin: 0; }
.diy-center p { margin: 0; }
.diy-save { position: sticky; bottom: 0; display: flex; align-items: center; justify-content: space-between; gap: 1rem; padding-block: .65rem; background: var(--canvas); }
/* Keep swipe and keyboard navigation without a visible scrollbar. */
.diy-tabs :deep([role="tablist"]) { scrollbar-width: none; }
.diy-tabs :deep([role="tablist"]::-webkit-scrollbar) { display: none; }
@media (prefers-reduced-motion: no-preference) {
  .diy-tabs :deep([role="tabpanel"][data-state="active"]) { animation: diy-enter .16s ease-out; }
  .diy-center :deep(button) { transition: color .14s, background-color .14s, opacity .14s, transform .14s; }
  .diy-center :deep(button:not(:disabled):not([aria-disabled="true"]):active) { transform: scale(.98); }
  .diy-feedback-enter-active, .diy-feedback-leave-active { transition: opacity .16s, transform .16s; }
  .diy-feedback-enter-from, .diy-feedback-leave-to { opacity: 0; transform: translateY(3px); }
}
@keyframes diy-enter { from { opacity: .55; transform: translateY(3px); } to { opacity: 1; transform: none; } }
@media(max-width:599px) { .diy-save { bottom: calc(var(--shell-safe-bottom, 0px)); flex-wrap: wrap; gap: .5rem; } .diy-save > div { margin-left: auto; } }
</style>
