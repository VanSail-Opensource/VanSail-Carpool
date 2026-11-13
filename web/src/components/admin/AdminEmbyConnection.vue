<script setup lang="ts">
import { onMounted, onScopeDispose, reactive, shallowRef } from 'vue'
import { embyConnectionApi, type EmbyConnection } from '@/api/embyConnection'
import { ApiError } from '@/api/http'
import InlineNotice from '@/components/common/InlineNotice.vue'
import { localizedError, useI18n } from '@/i18n'
import { createLatestRequest } from '@/utils/latestRequest'

const { t } = useI18n()
const current = shallowRef<EmbyConnection | null>(null)
const loading = shallowRef(true)
const busy = shallowRef(false)
const error = shallowRef<string | null>(null)
const draft = reactive({ baseURL: '', apiToken: '' })
const latest = createLatestRequest()
let disposed = false
async function load(): Promise<void> {
  if (disposed || busy.value) return
  const ticket = latest.begin()
  loading.value = true
  error.value = null
  try {
    const response = await embyConnectionApi.get()
    if (latest.isCurrent(ticket)) current.value = response
  }
  catch (caught) { if (latest.isCurrent(ticket)) error.value = localizedError(caught, 'errors.adminLoad') }
  finally { if (latest.isCurrent(ticket)) loading.value = false }
}
async function save(): Promise<boolean> {
  if (disposed || busy.value || loading.value || !current.value) return false
  if (!draft.baseURL.trim() && !draft.apiToken.trim()) return true
  error.value = null
  if (draft.baseURL.trim() && !draft.apiToken.trim()) {
    error.value = t('adminEmbyConnection.freshToken')
    return false
  }
  busy.value = true
  const ticket = latest.begin()
  const input = { version: current.value.version, baseURL: draft.baseURL.trim(), apiToken: draft.apiToken.trim() }
  try {
    const response = await embyConnectionApi.save(input)
    if (!latest.isCurrent(ticket)) return false
    current.value = response
    if (draft.baseURL.trim() === input.baseURL) draft.baseURL = ''
    if (draft.apiToken.trim() === input.apiToken) draft.apiToken = ''
    return true
  } catch (caught) {
    if (!latest.isCurrent(ticket)) return false
    error.value = caught instanceof ApiError && caught.code === 'EMBY_FRESH_TOKEN_REQUIRED'
      ? t('adminEmbyConnection.freshToken')
      : caught instanceof ApiError && caught.code === 'EMBY_CONNECTION_CHANGED'
        ? t('adminEmbyConnection.changed') : localizedError(caught, 'errors.adminAction')
    return false
  } finally { if (latest.isCurrent(ticket)) busy.value = false }
}
onMounted(() => void load())
onScopeDispose(() => {
  disposed = true
  latest.dispose()
  draft.baseURL = ''
  draft.apiToken = ''
  current.value = null
})
defineExpose({ save, loading })
</script>

<template>
  <fieldset class="emby-connection" :disabled="loading || busy">
    <legend>{{ t('adminEmbyConnection.title') }}</legend>
    <p>{{ t('adminEmbyConnection.hint') }}</p>
    <p v-if="loading" role="status">{{ t('common.loading') }}</p>
    <div v-else-if="current" class="emby-connection__fields">
      <UFormField :label="t('adminSettings.settingLabels.emby_base_url')" :hint="current.urlConfigured ? t('common.configured') : undefined">
        <UInput v-model="draft.baseURL" type="password" class="w-full" :aria-label="t('adminSettings.settingLabels.emby_base_url')" :placeholder="current.urlConfigured ? t('adminSettings.keepSecret') : ''" autocomplete="new-password" />
      </UFormField>
      <UFormField :label="t('adminSettings.settingLabels.emby_api_token')" :hint="current.tokenConfigured ? t('common.configured') : undefined">
        <UInput v-model="draft.apiToken" type="password" class="w-full" :aria-label="t('adminSettings.settingLabels.emby_api_token')" :placeholder="current.tokenConfigured ? t('adminSettings.keepSecret') : ''" autocomplete="new-password" />
      </UFormField>
    </div>
    <InlineNotice v-if="error" tone="warning">{{ error }}<UButton type="button" variant="link" :label="t('common.tryAgain')" @click="load" /></InlineNotice>
  </fieldset>
</template>

<style scoped>
.emby-connection { border: 0; padding: 1.25rem 0; margin: 0; }
.emby-connection legend, .emby-connection > p { color: var(--text-muted); font-size: .85rem; }
.emby-connection__fields { display: grid; gap: 1.35rem; margin-top: 1rem; }
@media (min-width: 1100px) { .emby-connection__fields { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
</style>
