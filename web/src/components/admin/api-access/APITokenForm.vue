<script setup lang="ts">
import { computed, reactive } from 'vue'
import { z } from 'zod'
import { apiTokenScopes, type PanelAPITokenWrite } from '@/api/adminAPITokens'
import { useI18n } from '@/i18n'

defineProps<{ busy: boolean }>()
const emit = defineEmits<{ create: [value: PanelAPITokenWrite] }>()
const { t } = useI18n()
const draft = reactive<PanelAPITokenWrite>({ name: '', scopes: ['xnode:write', 'migration:write'], expiresInDays: 30 })
const schema = computed(() => z.object({ name: z.string().trim().min(1, t('apiTokens.nameRequired')).max(80, t('apiTokens.nameRequired')), scopes: z.array(z.enum(apiTokenScopes)).min(1, t('apiTokens.scopeRequired')), expiresInDays: z.number().int().min(0).max(365) }))
const scopes = computed(() => apiTokenScopes.map(value => ({ value, label: t(`apiTokens.scopes.${value}`) })))
const expiry = computed(() => [7, 30, 90, 365, 0].map(value => ({ value, label: value ? t('apiTokens.days', { days: value }) : t('apiTokens.never') })))
const presets = computed(() => [
  { label: t('apiTokens.readOnly'), scopes: ['admin:read'] },
  { label: t('apiTokens.migration'), scopes: ['xnode:write', 'migration:write'] },
  { label: t('apiTokens.fullAccess'), scopes: ['admin:write'] },
] satisfies { label: string; scopes: PanelAPITokenWrite['scopes'] }[])
function submit(): void {
  const result = schema.value.safeParse(draft)
  if (result.success) emit('create', result.data)
}
</script>

<template>
  <UForm :schema="schema" :state="draft" class="grid gap-4" @submit="submit">
    <div class="grid gap-4 sm:grid-cols-2">
      <UFormField :label="t('apiTokens.name')" name="name"><UInput v-model="draft.name" :maxlength="80" :placeholder="t('apiTokens.namePlaceholder')" :disabled="busy" class="w-full" autocomplete="off" /></UFormField>
      <UFormField :label="t('apiTokens.expiration')" name="expiresInDays"><USelect v-model="draft.expiresInDays" :items="expiry" :disabled="busy" class="w-full" /></UFormField>
    </div>
    <div class="flex flex-wrap gap-2"><UButton v-for="preset in presets" :key="preset.label" type="button" color="neutral" variant="outline" size="sm" :label="preset.label" :disabled="busy" @click="draft.scopes = [...preset.scopes]" /></div>
    <UFormField :label="t('apiTokens.permissions')" :description="t('apiTokens.permissionsHint')" name="scopes"><UCheckboxGroup v-model="draft.scopes" :items="scopes" :disabled="busy" :ui="{ fieldset: 'grid gap-2 sm:grid-cols-2' }" /></UFormField>
    <div><UButton type="submit" icon="i-ph-plus" :label="t('apiTokens.create')" :loading="busy" /></div>
  </UForm>
</template>
