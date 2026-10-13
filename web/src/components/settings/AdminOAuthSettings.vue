<script setup lang="ts">
import { computed, onMounted, reactive, shallowRef } from 'vue'
import { z } from 'zod'
import { browserAccessApi, type OAuthSettings } from '@/api/browserAccess'
import { localizedError, useI18n } from '@/i18n'

const { t } = useI18n()
const settings = shallowRef<OAuthSettings | null>(null)
const loading = shallowRef(true)
const saving = shallowRef(false)
const error = shallowRef<string | null>(null)
const saved = shallowRef(false)
const draft = reactive({ enabled: false, clientId: '', clientSecret: '' })
const schema = computed(() => z.object({
  enabled: z.boolean(), clientId: z.string(), clientSecret: z.string().max(512, t('browser.oauth.secretTooLong')),
}).superRefine((value, ctx) => {
  if ((value.enabled || value.clientId !== '') && !/^[1-9]\d*$/.test(value.clientId)) ctx.addIssue({ code: 'custom', path: ['clientId'], message: t('browser.oauth.clientRequired') })
  if (value.enabled && !value.clientSecret && !settings.value?.clientSecretConfigured) ctx.addIssue({ code: 'custom', path: ['clientSecret'], message: t('browser.oauth.secretRequired') })
}))
async function load(): Promise<void> {
  loading.value = true
  error.value = null
  try {
    settings.value = await browserAccessApi.getOAuthSettings()
    draft.enabled = settings.value.enabled
    draft.clientId = settings.value.clientId
  } catch (caught) { error.value = localizedError(caught, 'browser.loadFailed') }
  finally { loading.value = false }
}
async function save(): Promise<void> {
  if (saving.value) return
  saving.value = true
  error.value = null
  saved.value = false
  try {
    settings.value = await browserAccessApi.saveOAuthSettings({ ...draft })
    draft.clientSecret = ''
    saved.value = true
  } catch (caught) { error.value = localizedError(caught, 'browser.saveFailed') }
  finally { saving.value = false }
}
onMounted(() => void load())
</script>

<template>
  <section class="oauth-settings">
    <header><h2>{{ t('browser.oauth.title') }}</h2><p>{{ t('browser.oauth.description') }}</p></header>
    <UAlert v-if="error" color="warning" variant="soft" :title="error"><template #actions><UButton v-if="!settings" :label="t('common.tryAgain')" variant="link" @click="load" /></template></UAlert>
    <USkeleton v-if="loading" class="h-48" />
    <UForm v-else-if="settings" :state="draft" :schema="schema" class="oauth-settings-form" @submit="save">
      <UFormField name="enabled"><USwitch v-model="draft.enabled" :label="t('browser.oauth.enabled')" :disabled="saving" /></UFormField>
      <UFormField name="clientId" :label="t('browser.oauth.clientId')"><UInput v-model.trim="draft.clientId" inputmode="numeric" autocomplete="off" :disabled="saving" class="w-full" /></UFormField>
      <UFormField name="clientSecret" :label="t('browser.oauth.clientSecret')" :description="t(settings.clientSecretConfigured ? 'browser.oauth.secretStored' : 'browser.oauth.secretNeeded')"><UInput v-model="draft.clientSecret" type="password" autocomplete="new-password" :disabled="saving" class="w-full" /></UFormField>
      <UFormField :label="t('browser.oauth.callbackUrl')" :description="t('browser.oauth.registration')"><UInput :model-value="settings.callbackUrl" readonly class="w-full" /></UFormField>
      <div class="button-row"><UButton type="submit" :loading="saving" :label="t('common.save')" /><span v-if="saved" role="status">{{ t('browser.saved') }}</span></div>
    </UForm>
  </section>
</template>

<style scoped>
.oauth-settings { min-width: 0; }
.oauth-settings header { margin-bottom: 20px; }
.oauth-settings h2 { margin: 0; font-size: 20px; }
.oauth-settings header p { margin: 8px 0 0; color: var(--text-muted); font-size: 13px; }
.oauth-settings-form { display: grid; gap: 18px; max-width: 600px; }
.oauth-settings .button-row { display: flex; align-items: center; gap: 12px; }
.oauth-settings .button-row span { font-size: 12px; color: var(--success); }
</style>
