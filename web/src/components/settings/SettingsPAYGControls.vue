<script setup lang="ts">
import { computed } from 'vue'
import { paygApi } from '@/api/payg'
import InlineNotice from '@/components/common/InlineNotice.vue'
import { useSubscriptionData } from '@/components/subscription/useSubscriptionData'
import { usePreferencesStore } from '@/stores/preferences'
import SettingsSwitchRow from './SettingsSwitchRow.vue'

const preferences = usePreferencesStore()
const { value, loading, error, refresh } = useSubscriptionData(paygApi.get, 60_000, 'payg.loadFailed')
const squads = computed(() => value.value?.squads.filter(item => item.squad.billingMode === 'payg') ?? [])
const enabled = computed(() => preferences.value?.paygEnabled ?? true)
</script>

<template>
  <div class="settings-payg">
    <h3>{{ $t('payg.title') }}</h3>
    <SettingsSwitchRow :label="$t('settings.payg.enabled')" :description="$t('settings.payg.hint')" :enabled="enabled" :disabled="!preferences.value || preferences.saving" :saving="preferences.pendingKey === 'paygEnabled'" @update="preferences.save({ paygEnabled: $event })" />
    <USkeleton v-if="loading" class="h-12" />
    <SettingsSwitchRow v-for="item in squads" :key="item.squad.groupId" :label="item.squad.name" :enabled="preferences.value?.paygSquads[item.squad.groupId] ?? true" :disabled="!enabled || !preferences.value || preferences.saving" @update="preferences.save({ paygSquads: { [item.squad.groupId]: $event } })" />
    <InlineNotice v-if="error" tone="warning">{{ error }}<UButton variant="link" :label="$t('common.tryAgain')" data-haptic="retry" @click="refresh()" /></InlineNotice>
  </div>
</template>

<style scoped>
.settings-payg { min-width: 0; padding-top: .75rem; }
.settings-payg h3 { margin: 0; color: var(--text-muted); font-size: .875rem; font-weight: 600; }
</style>
