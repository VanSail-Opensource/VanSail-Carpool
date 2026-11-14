<script setup lang="ts">
import LoadingPlaceholder from '@/components/common/LoadingPlaceholder.vue'
import { computed, shallowRef, watch } from 'vue'
import { selectionHaptic } from '@/utils/telegramHaptics'
import { useI18n } from '@/i18n'
import MemberRoutingEditor from '@/components/subscription/MemberRoutingEditor.vue'
import OperationStatusNotice from '@/components/common/OperationStatusNotice.vue'
import DIYTuningPanel from './DIYTuningPanel.vue'
import DIYCompatibility from './DIYCompatibility.vue'
import DIYSplitProfiles from './DIYSplitProfiles.vue'
import RoutingDemoButton from './RoutingDemoButton.vue'
import { useDIY } from './useDIY'
import { useScrollableTabs } from './useScrollableTabs'

const { t } = useI18n()
const state = useDIY()
const tabIcons = { connection: 'i-ph-plugs-connected', xhttp: 'i-ph-arrows-left-right', routing: 'i-ph-path', compatibility: 'i-ph-devices' }
const tabs = computed(() => Object.entries(tabIcons).map(([value, icon]) => ({ value, icon, label: t(`diy.tabs.${value}`) })))
const activeTab = shallowRef<string | number>('connection')
const tabRoot = shallowRef<InstanceType<typeof globalThis.HTMLElement> | null>(null)
const scrolling = useScrollableTabs(tabRoot)
watch([activeTab, tabs], scrolling.revealSelected, { flush: 'post' })
function selectTab(value: string | number) { if (value !== activeTab.value) selectionHaptic(); activeTab.value = value }
const routePending = computed(() => state.view.value && !['active', 'disabled'].includes(state.view.value.routing.plan.phase))
const routingError = computed(() => state.errors.value.routing ? t(`managedRouting.errors.${state.errors.value.routing}`) : '')
</script>

<template>
  <div class="page diy-center">
    <header class="diy-header">
      <h1><UIcon name="i-ph-sliders-horizontal" aria-hidden="true" />{{ t('diy.title') }}</h1>
      <div class="diy-header-actions">
        <UPopover><UButton icon="i-ph-info" color="neutral" variant="ghost" :aria-label="t('diy.about')" /><template #content><p class="max-w-xs p-3 text-sm">{{ t('diy.description') }}</p></template></UPopover>
        <UButton icon="i-ph-arrow-clockwise" variant="ghost" color="neutral" data-haptic="refresh" :aria-label="t('common.refresh')" :loading="state.loading.value" :disabled="state.submitting.value" @click="state.load(true)" />
      </div>
    </header>
    <LoadingPlaceholder v-if="state.loading.value || state.view.value?.inventoryPending" :variant="state.view.value ? 'compact' : 'form'" :rows="6" />
    <UAlert v-if="state.error.value" color="warning" :title="state.error.value" />
    <UButton v-if="state.error.value && !state.view.value" class="justify-self-start" variant="outline" :label="t('common.tryAgain')" @click="state.load(true)" />
    <template v-if="state.view.value">
      <div ref="tabRoot" class="diy-categories">
        <UTabs class="diy-tabs" :items="tabs" :model-value="activeTab" @update:model-value="selectTab" :unmount-on-hide="false" variant="pill" color="neutral" size="sm" :ui="{ root: 'w-full min-w-0 gap-5', list: 'w-full min-w-0 max-w-full overflow-x-auto p-1 rounded-xl', trigger: 'shrink-0 flex-1 min-h-11 px-3 gap-2', leadingIcon: 'size-4 shrink-0', label: 'shrink-0 whitespace-nowrap text-xs sm:text-sm', content: 'min-w-0' }">
          <template #content="{ item }">
            <div v-if="['connection', 'xhttp'].includes(item.value)" class="grid gap-4 min-w-0">
              <DIYSplitProfiles v-if="item.value === 'xhttp'" v-model="state.splitProfiles.value" :hosts="state.view.value.splitHosts ?? []" :disabled="state.submitting.value" :error="state.errors.value.splitProfiles ? t(`diy.errors.${state.errors.value.splitProfiles}`) : undefined" />
              <DIYTuningPanel v-model="state.overrides.value" :fields="state.view.value.fields" :groups="item.value === 'connection' ? ['tls', 'shadowsocks', 'mux', 'singboxMux', 'mihomoMux'] : ['xhttp', 'xmux']" :errors="state.errors.value" :disabled="state.submitting.value" />
              <UAlert v-if="state.errors.value.overrides" color="warning" :title="t(`diy.errors.${state.errors.value.overrides}`)" />
            </div>
            <div v-else-if="item.value === 'routing'" class="grid gap-4 min-w-0">
              <div class="diy-routing-toolbar"><span class="diy-routing-label"><UIcon name="i-ph-path" aria-hidden="true" />{{ t('diy.routing') }}</span><RoutingDemoButton /></div>
              <MemberRoutingEditor external :initial="state.view.value.routing" :disabled="state.submitting.value" :preview-paused="state.loading.value || !state.confirmed.value" @draft="state.routeDraft" @refresh="state.load(true)" @save="state.save" />
              <UAlert v-if="routingError" color="warning" :title="routingError" />
            </div>
            <DIYCompatibility v-else-if="item.value === 'compatibility'" :items="state.compatibility.value" />
          </template>
        </UTabs>
        <span v-if="scrolling.before.value" class="diy-overflow diy-overflow--before" aria-hidden="true"><UIcon name="i-ph-caret-left-bold" /></span>
        <span v-if="scrolling.after.value" class="diy-overflow diy-overflow--after" aria-hidden="true"><UIcon name="i-ph-caret-right-bold" /></span>
      </div>
      <LoadingPlaceholder v-if="state.autoPreview.pending.value" variant="compact" />
      <UAlert v-if="state.autoPreview.error.value" color="warning" :title="state.autoPreview.error.value" />
      <OperationStatusNotice :receipt="state.tracker.receipt.value" :error="state.tracker.error.value" :checking="state.tracker.checking.value" @refresh="state.tracker.refresh" />
      <Transition name="diy-feedback"><p v-if="state.saved.value" role="status" class="diy-saved text-sm text-success"><UIcon :name="routePending ? 'i-ph-clock-countdown' : 'i-ph-check-circle'" aria-hidden="true" />{{ t(routePending ? 'diy.routingPending' : 'diy.saved') }}</p></Transition>
      <div class="diy-save">
        <div class="diy-save-actions"><UButton data-haptic="action" variant="ghost" color="neutral" icon="i-ph-arrow-u-up-left" :label="t('diy.reset')" :title="t('diy.resetHint')" :disabled="state.submitting.value" @click="state.reset" /><UButton data-haptic="confirm" icon="i-ph-floppy-disk" :label="t('common.save')" :loading="state.submitting.value" :disabled="state.submitting.value || state.loading.value || !state.confirmed.value || state.invalid.value || state.autoPreview.pending.value" @click="state.save" /></div>
      </div>
    </template>
  </div>
</template>

<style scoped>
.diy-center { display: grid; grid-template-columns: minmax(0, 1fr); gap: 1rem; max-width: 1000px; margin-inline: auto; padding-bottom: var(--mobile-nav-clearance, 2rem); min-width: 0; }
.diy-header, .diy-header-actions, .diy-routing-toolbar, .diy-routing-label, .diy-saved { display: flex; align-items: center; gap: .6rem; }
.diy-header, .diy-routing-toolbar { justify-content: space-between; }
.diy-header-actions { gap: .25rem; }
.diy-center h1 { display: flex; align-items: center; gap: .6rem; font-size: var(--page-title-size); font-weight: 650; letter-spacing: var(--page-title-tracking); line-height: 1.2; margin: 0; }
.diy-center h1 :deep(.iconify) { font-size: 1.3rem; color: var(--text-muted); }
.diy-routing-label { font-size: .85rem; color: var(--text-muted); }
.diy-center p { margin: 0; }
.diy-save { display: flex; justify-content: flex-end; padding-block: .8rem; border-top: 1px solid var(--line); background: var(--canvas); }
.diy-save-actions { display: flex; gap: .5rem; }
.diy-tabs { min-width: 0; max-width: 100%; }
.diy-categories { position: relative; min-width: 0; }
.diy-tabs :deep([role="tablist"]) { scrollbar-width: thin; scrollbar-color: var(--line) transparent; scroll-padding-inline: .5rem; }
.diy-overflow { position: absolute; top: .25rem; height: 2.75rem; width: .8rem; display: grid; place-items: center; pointer-events: none; color: var(--text-muted); background: var(--surface-raised); }
.diy-overflow--before { left: 0; }
.diy-overflow--after { right: 0; }
@media (prefers-reduced-motion: no-preference) {
  .diy-tabs :deep([role="tabpanel"][data-state="active"]) { animation: diy-enter .16s ease-out; }
  .diy-center :deep(button) { transition: color .14s, background-color .14s, opacity .14s, transform .14s; }
  .diy-center :deep(button:not(:disabled):not([aria-disabled="true"]):active) { transform: scale(.98); }
  .diy-feedback-enter-active, .diy-feedback-leave-active { transition: opacity .16s, transform .16s; }
  .diy-feedback-enter-from, .diy-feedback-leave-to { opacity: 0; transform: translateY(3px); }
}
@keyframes diy-enter { from { opacity: .55; transform: translateY(3px); } to { opacity: 1; transform: none; } }
@media(max-width:599px) {
  .diy-save-actions { display: grid; width: 100%; grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .diy-save-actions :deep(button) { min-width: 0; }
}
</style>
