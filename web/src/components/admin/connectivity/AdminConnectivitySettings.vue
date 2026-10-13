<script setup lang="ts">
import { computed, shallowRef } from 'vue'
import InlineNotice from '@/components/common/InlineNotice.vue'
import { useI18n } from '@/i18n'
import ConnectivityConfigForm from './ConnectivityConfigForm.vue'
import ConnectivityHistory from './ConnectivityHistory.vue'
import ConnectivityResults from './ConnectivityResults.vue'
import { useConnectivityMonitor } from './useConnectivityMonitor'
import { useConnectivitySettings } from './useConnectivitySettings'

const { t } = useI18n()
const { snapshot, loading, checking, error: monitorError, active, refresh, check } = useConnectivityMonitor()
const { draft, username, user, busy, resolving, error, saved, dirty, valid, canRun, resolve, clearUser, save } = useConnectivitySettings(snapshot, refresh)
const workspace = shallowRef('results')
const tabs = computed(() => [{ value: 'results', label: t('adminWorkspace.results') }, { value: 'history', label: t('hostConnectivity.history') }, { value: 'config', label: t('adminWorkspace.configuration') }])
const controlsBusy = computed(() => busy.value || checking.value)
</script>

<template>
  <section class="admin-panel connectivity-settings" aria-labelledby="connectivity-settings-title">
    <div class="admin-panel__heading">
      <div><h2 id="connectivity-settings-title">{{ t('hostConnectivity.title') }}</h2><p>{{ t('hostConnectivity.copy') }}</p></div>
      <div class="connectivity-settings__actions">
        <UButton color="neutral" variant="outline" icon="i-ph-play-fill" :label="active ? t('hostConnectivity.checking') : t('hostConnectivity.runNow')" :loading="checking || active" :disabled="loading || !canRun || active || checking" @click="check" />
        <UButton icon="i-ph-floppy-disk" :label="t('adminSettings.save')" :loading="busy" :disabled="loading || controlsBusy || resolving || !dirty || !valid" @click="save" />
      </div>
    </div>
    <UTabs v-model="workspace" :items="tabs" variant="link" :content="false" class="admin-workspace-tabs" />
    <USkeleton v-if="loading" class="connectivity-settings__skeleton" />
    <template v-else-if="snapshot">
      <ConnectivityConfigForm v-show="workspace === 'config'" :config="draft" :username="username" :user="user" :disabled="controlsBusy" :resolving="resolving" @change="Object.assign(draft, $event)" @username="username = $event" @resolve="resolve" @clear="clearUser" />
      <InlineNotice v-if="error" tone="warning">{{ error }}</InlineNotice>
      <InlineNotice v-else-if="dirty && !valid" tone="warning">{{ t('hostConnectivity.invalidDraft') }}</InlineNotice>
      <small v-else-if="dirty" class="connectivity-settings__hint">{{ t('hostConnectivity.saveBeforeRun') }}</small>
      <InlineNotice v-if="saved && !dirty" tone="success">{{ t('hostConnectivity.saved') }}</InlineNotice>
      <ConnectivityResults v-show="workspace === 'results'" :snapshot="snapshot" />
    </template>
    <InlineNotice v-if="monitorError" tone="warning">{{ monitorError }}</InlineNotice>
    <UButton v-if="monitorError" color="neutral" variant="outline" :label="t('adminSection.retry')" @click="refresh" />
    <ConnectivityHistory v-show="workspace === 'history'" :snapshot="snapshot" />
  </section>
</template>

<style scoped>
.connectivity-settings { display: grid; gap: 0.85rem; padding: 0; }
.connectivity-settings__actions { display: flex; flex-wrap: wrap; gap: 0.5rem; }
.connectivity-settings__actions :deep(button) { min-height: 44px; }
.connectivity-settings__skeleton { height: 13rem; width: 100%; }
.connectivity-settings__hint { color: var(--text-muted); }
</style>
