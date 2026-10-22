<script setup lang="ts">
import { onMounted } from 'vue'
import { useRouter } from 'vue-router'
import InlineNotice from '@/components/common/InlineNotice.vue'
import LanguageControl from '@/components/layout/LanguageControl.vue'
import CurrencyControl from '@/components/layout/CurrencyControl.vue'
import TrafficResetAutomationControl from '@/components/dashboard/TrafficResetAutomationControl.vue'
import { usePreferencesStore } from '@/stores/preferences'
import { useSessionStore } from '@/stores/session'
import NotificationSettings from './NotificationSettings.vue'
import SettingsSwitchRow from './SettingsSwitchRow.vue'
import GroupMemberTagEditor from './GroupMemberTagEditor.vue'
import SettingsComboControls from './SettingsComboControls.vue'
import { useSettings } from './useSettings'

const preferences = usePreferencesStore()
const session = useSessionStore()
const settings = useSettings()
const router = useRouter()
onMounted(() => void preferences.refresh())

function navigate(path: string): void {
  void router.push(path).catch(() => undefined)
}
</script>

<template>
  <div class="settings-page">
    <header class="page-header"><h1>{{ $t('settings.title') }}</h1></header>
    <UButton v-if="session.isAdmin" color="neutral" variant="outline" icon="i-ph-shield-check-bold" :label="$t('browser.adminAccess')" @click="navigate('/settings/admin-access')" />
    <UButton color="neutral" variant="outline" icon="i-ph-sliders-horizontal-bold" :label="$t('diy.title')" @click="navigate('/settings/diy')" />
    <InlineNotice v-if="preferences.error" tone="warning">{{ preferences.error }}<UButton variant="link" :label="$t('common.tryAgain')" @click="preferences.refresh()" /></InlineNotice>
    <USkeleton v-if="preferences.loading && !preferences.value" class="h-48" />
    <NotificationSettings />
    <section class="settings-section">
      <h2>{{ $t('settings.subscription') }}</h2>
      <TrafficResetAutomationControl :enabled="settings.automation.value?.enabled ?? null" :loading="settings.automationLoading.value" :saving="settings.automationSaving.value" :error="settings.automationError.value" @update="settings.setAutomation" />
      <UButton v-if="settings.automationError.value" variant="link" :label="$t('common.tryAgain')" @click="settings.loadAutomation()" />
      <SettingsComboControls />
    </section>
    <section class="settings-section">
      <h2>{{ $t('settings.display') }}</h2>
      <template v-if="preferences.value?.activeCombo">
        <SettingsSwitchRow :label="$t('settings.showAroundTx')" :enabled="preferences.value.showAroundTx" :disabled="preferences.saving" @update="preferences.save({ showAroundTx: $event })" />
        <SettingsSwitchRow :label="$t('settings.showActivity')" :enabled="preferences.value.showActivity" :disabled="preferences.saving" @update="preferences.save({ showActivity: $event })" />
      </template>
      <SettingsSwitchRow :label="$t('settings.includeNodePrices')" :description="$t('settings.includeNodePricesHint')" :enabled="preferences.includeNodePrices" :disabled="!preferences.value || preferences.saving" @update="preferences.save({ includeNodePrices: $event })" />
      <div class="settings-selectors"><LanguageControl show-label /><CurrencyControl show-label /></div>
    </section>
    <section class="settings-section">
      <h2>{{ $t('settings.community') }}</h2>
      <SettingsSwitchRow :label="$t('settings.showReferralUsername')" :description="$t('settings.showReferralUsernameHint')" :enabled="preferences.value?.showReferralUsername ?? true" :disabled="!preferences.value || preferences.saving" @update="preferences.save({ showReferralUsername: $event })" />
      <USkeleton v-if="settings.tagLoading.value" class="h-12" />
      <GroupMemberTagEditor v-else-if="settings.tag.value?.groupJoined" :state="settings.tag.value" :saving="settings.tagSaving.value" :error="settings.tagError.value" @save="settings.saveTag" />
      <InlineNotice v-if="settings.tagError.value && !settings.tag.value?.groupJoined" tone="warning">{{ settings.tagError.value }}<UButton variant="link" :label="$t('common.tryAgain')" @click="settings.loadTag()" /></InlineNotice>
    </section>
  </div>
</template>

<style>
.settings-page { width: min(100%, 44rem); margin-inline: auto; padding: 1rem 1.15rem 3rem; }
.settings-section { padding-block: 1.25rem; }
.settings-section + .settings-section { border-top: 1px solid var(--line); }
.settings-section h2 { margin: 0 0 0.75rem; color: var(--text-muted); font-size: 0.875rem; font-weight: 600; }
.settings-selectors { display: grid; gap: 0; }
</style>
