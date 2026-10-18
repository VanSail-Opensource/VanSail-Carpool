<script setup lang="ts">
import { computed, shallowRef, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import InlineNotice from '@/components/common/InlineNotice.vue'
import OperationStatusNotice from '@/components/common/OperationStatusNotice.vue'
import SettingsSwitchRow from './SettingsSwitchRow.vue'
import EarlyActivationDialog from './EarlyActivationDialog.vue'
import SquadAdditionDialog from '@/components/squad-addition/SquadAdditionDialog.vue'
import { useComboControls } from './useComboControls'
import { useI18n } from '@/i18n'

const controls = useComboControls()
const { t } = useI18n()
const open = shallowRef(false)
const additionOpen = shallowRef(false)
const route = useRoute()
const router = useRouter()
watch([() => route.query.addSquads, controls.value], ([requested, value]) => {
  if (requested !== '1' || !value?.activePurchase || value.queuedPurchase || !value.mutable) return
  additionOpen.value = true
  const query = { ...route.query }
  delete query.addSquads
  void router.replace({ query })
})
const message = computed(() => {
  const status = controls.command.receipt.value?.status
  return t(status === 'succeeded' ? 'settings.combo.updated' : status === 'failed' || status === 'pending_review' || status === 'partial' ? 'settings.combo.syncFailed' : 'settings.combo.syncPending')
})
async function activate(confirmation: string): Promise<void> { if (await controls.activate(confirmation)) open.value = false }
</script>

<template>
  <div class="combo-controls">
    <USkeleton v-if="controls.loading.value" class="h-24" />
    <template v-else-if="controls.value.value?.activePurchase">
      <UButton type="button" color="neutral" variant="ghost" class="combo-controls__add" :aria-label="$t('home.squadAddition.open')" :disabled="controls.blocked.value || Boolean(controls.value.value.queuedPurchase)" data-haptic="open" @click="additionOpen = true"><span><strong>{{ $t('home.squadAddition.open') }}</strong><small>{{ $t('home.squadAddition.description') }}</small></span><UIcon name="i-ph-caret-right" aria-hidden="true" /></UButton>
      <div class="combo-controls__heading"><h3>{{ $t('settings.combo.squads') }}</h3><UButton variant="ghost" color="neutral" icon="i-ph-arrow-clockwise" :aria-label="$t('common.refresh')" :loading="controls.refreshing.value" @click="controls.load" /></div>
      <InlineNotice v-if="!controls.value.value.mutable && !controls.command.blocksMutations.value" tone="info">{{ $t('settings.combo.accessPending') }}</InlineNotice>
      <SettingsSwitchRow v-for="squad in controls.value.value.squads" :key="squad.uuid" :label="squad.name" :enabled="squad.enabled" :description="squad.enabled && !squad.canDisable && !controls.blocked.value ? $t('settings.combo.keepOne') : undefined" :disabled="controls.blocked.value || (squad.enabled && !squad.canDisable)" @update="controls.switchSquad(squad.uuid, $event)" />
      <div v-if="controls.quote.value?.eligible" class="combo-controls__action">
        <div><strong>{{ $t('settings.combo.activateTitle') }}</strong><p>{{ $t('settings.combo.activateHint') }}</p></div>
        <UButton color="neutral" variant="ghost" trailing-icon="i-ph-arrow-right" :label="$t('settings.combo.activate')" :disabled="controls.blocked.value" @click="open = true" />
      </div>
    </template>
    <OperationStatusNotice v-if="controls.command.receipt.value" :receipt="controls.command.receipt.value" :error="controls.error.value" :checking="controls.command.checking.value" :message="message" @refresh="controls.command.refresh" />
    <InlineNotice v-else-if="controls.error.value" tone="warning">{{ controls.error.value }}<UButton variant="link" :label="$t('common.tryAgain')" @click="controls.load" /></InlineNotice>
    <EarlyActivationDialog v-model:open="open" :quote="controls.quote.value" :busy="controls.command.busy.value" @confirm="activate" />
    <SquadAdditionDialog v-if="controls.value.value?.activePurchase" v-model:open="additionOpen" :active="controls.value.value.activePurchase" @changed="controls.load" />
  </div>
</template>

<style scoped>
.combo-controls { min-width: 0; padding-top: 0.75rem; }
.combo-controls h3 { margin: 0; padding-top: 0.75rem; color: var(--text-muted); font-size: 0.875rem; font-weight: 600; }
.combo-controls__heading { display: flex; justify-content: space-between; align-items: center; gap: 0.5rem; }
.combo-controls__action { display: flex; align-items: center; justify-content: space-between; gap: 1rem; min-height: 60px; padding-block: 0.75rem; }
.combo-controls__action > div { min-width: 0; flex: 1; }
.combo-controls__action strong { font-size: 0.875rem; }
.combo-controls__action p { margin: 0.25rem 0 0; color: var(--text-muted); font-size: 0.8rem; line-height: 1.45; }
.combo-controls__action :deep(button) { flex-shrink: 0; }
.combo-controls__add { width: 100%; display: flex; align-items: center; justify-content: space-between; gap: 1rem; min-height: 60px; padding: 0.75rem 0; border: 0; background: transparent; color: var(--text); text-align: left; cursor: pointer; }
.combo-controls__add span { min-width: 0; display: grid; gap: 0.25rem; }
.combo-controls__add strong { font-size: 0.875rem; font-weight: 600; }
.combo-controls__add small { color: var(--text-muted); font-size: 0.8rem; line-height: 1.45; }
.combo-controls__add:disabled { opacity: 0.5; cursor: not-allowed; }
.combo-controls__add:focus-visible { outline: 2px solid var(--accent); outline-offset: 3px; border-radius: var(--radius-control); }
</style>
