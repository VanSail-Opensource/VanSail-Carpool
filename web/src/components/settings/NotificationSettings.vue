<script setup lang="ts">
import type { UserPreferences } from '@/api/preferences'
import { usePreferencesStore } from '@/stores/preferences'
import SettingsSwitchRow from './SettingsSwitchRow.vue'

const store = usePreferencesStore()
const categories: readonly (keyof UserPreferences['notifications'])[] = ['combos', 'traffic', 'money', 'activity', 'account']
</script>

<template>
  <section class="settings-section">
    <h2>{{ $t('settings.notifications.title') }}</h2>
    <SettingsSwitchRow v-for="category in categories" :key="category" :label="$t(`settings.notifications.${category}`)" :description="$t(`settings.notifications.${category}Hint`)" :enabled="store.value?.notifications[category] ?? true" :disabled="!store.value || store.blocked" :saving="store.pendingKey === `notifications.${category}`" @update="store.save({ notifications: { [category]: $event } })" />
  </section>
</template>
