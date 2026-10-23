<script setup lang="ts">
import type { ConnectivityConfig, ConnectivityUser } from '@/api/connectivity'
import SwitchField from '@/components/common/SwitchField.vue'
import { useI18n } from '@/i18n'

defineProps<{ config: ConnectivityConfig; username: string; user: ConnectivityUser | null; disabled: boolean; resolving: boolean }>()
const emit = defineEmits<{ change: [value: Partial<ConnectivityConfig>]; username: [value: string]; resolve: []; clear: [] }>()
const { t } = useI18n()
</script>

<template>
  <form class="connectivity-config" @submit.prevent="emit('resolve')">
    <UFormField name="connectivity-username" :label="t('hostConnectivity.username')" :description="t('hostConnectivity.usernameHint')">
      <div class="connectivity-config__account">
        <UInput id="connectivity-username" :model-value="username" class="w-full" autocomplete="off" :maxlength="100" :disabled="disabled" @update:model-value="emit('username', String($event))" />
        <UButton type="submit" color="neutral" variant="outline" :label="t('hostConnectivity.resolve')" :loading="resolving" :disabled="disabled || !username.trim() || resolving" />
      </div>
    </UFormField>
    <div v-if="config.proxyUserId > 0" class="connectivity-config__selected">
      <span>{{ user?.id === config.proxyUserId ? t('hostConnectivity.selectedUser', { name: user.username, id: user.id }) : t('hostConnectivity.selectedId', { id: config.proxyUserId }) }}</span>
      <UButton type="button" color="neutral" variant="link" :label="t('hostConnectivity.clearUser')" :disabled="disabled" @click="emit('clear')" />
    </div>
    <SwitchField id="connectivity-scheduled" :model-value="config.scheduledEnabled" :label="t('hostConnectivity.scheduled')" :help="t('hostConnectivity.scheduledHint')" :disabled="disabled || config.proxyUserId === 0" @update:model-value="emit('change', { scheduledEnabled: $event })" />
    <div class="connectivity-config__timings">
      <UFormField name="connectivity-interval" :label="t('hostConnectivity.interval')" :description="t('hostConnectivity.intervalHint')">
        <UInput id="connectivity-interval" class="w-full" type="number" inputmode="numeric" :model-value="config.intervalSeconds" :min="60" :max="86400" :step="1" :disabled="disabled" @update:model-value="emit('change', { intervalSeconds: Number($event) })" />
      </UFormField>
      <UFormField name="connectivity-timeout" :label="t('hostConnectivity.timeout')" :description="t('hostConnectivity.timeoutHint')">
        <UInput id="connectivity-timeout" class="w-full" type="number" inputmode="numeric" :model-value="config.timeoutSeconds" :min="1" :max="60" :step="1" :disabled="disabled" @update:model-value="emit('change', { timeoutSeconds: Number($event) })" />
      </UFormField>
      <UFormField name="connectivity-max-retries" :label="t('hostConnectivity.maxRetries')" :description="t('hostConnectivity.maxRetriesHint')">
        <UInput id="connectivity-max-retries" class="w-full" type="number" inputmode="numeric" :model-value="config.maxRetries" :min="0" :max="10" :step="1" :disabled="disabled" @update:model-value="emit('change', { maxRetries: Number($event) })" />
      </UFormField>
      <UFormField name="connectivity-retry-interval" :label="t('hostConnectivity.retryInterval')" :description="t('hostConnectivity.retryIntervalHint')">
        <UInput id="connectivity-retry-interval" class="w-full" type="number" inputmode="numeric" :model-value="config.retryIntervalSeconds" :min="1" :max="60" :step="1" :disabled="disabled" @update:model-value="emit('change', { retryIntervalSeconds: Number($event) })" />
      </UFormField>
    </div>
    <UFormField name="connectivity-probe-url" :label="t('hostConnectivity.probeUrl')" :description="t('hostConnectivity.probeUrlHint')">
      <UInput id="connectivity-probe-url" class="w-full" type="url" :model-value="config.probeUrl" :maxlength="2048" :disabled="disabled" @update:model-value="emit('change', { probeUrl: String($event) })" />
    </UFormField>
  </form>
</template>

<style scoped>
.connectivity-config { display: grid; gap: 0.85rem; }
.connectivity-config__account { display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: 0.5rem; }
.connectivity-config__selected { display: flex; align-items: center; justify-content: space-between; gap: 0.5rem; color: var(--text-muted); font-size: 0.8rem; }
.connectivity-config__selected span { overflow-wrap: anywhere; }
.connectivity-config__timings { display: grid; gap: 0.85rem; grid-template-columns: repeat(auto-fit, minmax(min(100%, 14rem), 1fr)); }
</style>
