<script setup lang="ts">
import { computed } from 'vue'
import { paygApi } from '@/api/payg'
import InlineNotice from '@/components/common/InlineNotice.vue'
import { useSubscriptionData } from '@/components/subscription/useSubscriptionData'
import { usePreferencesStore } from '@/stores/preferences'
import SettingsSwitchRow from './SettingsSwitchRow.vue'

const preferences = usePreferencesStore()
const { value, loading, refreshing, liveReady, error, refresh } = useSubscriptionData(paygApi.get, 60_000, 'payg.loadFailed', undefined, '/api/v1/payg')
const squads = computed(() => value.value?.squads.filter(item => item.squad.billingMode === 'payg') ?? [])
const enabled = computed(() => preferences.value?.paygEnabled ?? true)
const balanceMinor = computed(() => value.value?.balance.minor)
const balanceKnown = computed(() => balanceMinor.value != null && /^-?\d+$/.test(balanceMinor.value))
const balancePositive = computed(() => balanceKnown.value && BigInt(balanceMinor.value!) > 0n)
const controlsDisabled = computed(() => !preferences.value || preferences.blocked || preferences.saving || !liveReady.value || refreshing.value || !!error.value || !balancePositive.value)
</script>

<template>
  <div class="settings-payg">
    <h3>{{ $t('payg.title') }}</h3>
    <SettingsSwitchRow :label="$t('settings.payg.enabled')" :description="$t('settings.payg.hint')" :enabled="enabled" :disabled="controlsDisabled" :saving="preferences.pendingKey === 'paygEnabled'" @update="preferences.save({ paygEnabled: $event })" />
    <InlineNotice v-if="balanceKnown && !balancePositive && !error" tone="warning">{{ $t('settings.payg.balanceRequired') }}</InlineNotice>
    <USkeleton v-if="loading" class="h-12" />
    <SettingsSwitchRow v-for="item in squads" :key="item.squad.groupId" :label="item.squad.name" :enabled="preferences.value?.paygSquads[item.squad.groupId] ?? true" :disabled="!enabled || controlsDisabled" @update="preferences.save({ paygSquads: { [item.squad.groupId]: $event } })" />
    <InlineNotice v-if="error" tone="warning">{{ error }}<UButton variant="link" :label="$t('common.tryAgain')" data-haptic="retry" @click="refresh()" /></InlineNotice>
  </div>
</template>

<style scoped>
.settings-payg { min-width: 0; padding-top: .75rem; }
.settings-payg h3 { margin: 0; color: var(--text-muted); font-size: .875rem; font-weight: 600; }
</style>
