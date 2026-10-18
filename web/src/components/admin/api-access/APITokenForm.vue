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
const groups = computed(() => ['admin', 'xnode', 'migration'].map(group => ({ group, label: t(`apiTokens.groups.${group}`), scopes: apiTokenScopes.filter(value => value.startsWith(`${group}:`)) })))
function toggle(scope: PanelAPITokenWrite['scopes'][number], checked: boolean): void {
  draft.scopes = checked ? [...new Set([...draft.scopes, scope])] : draft.scopes.filter(value => value !== scope)
}
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
    <fieldset class="grid gap-2">
      <legend class="text-sm font-medium mb-2">{{ t('apiTokens.permissions') }}</legend>
      <UFormField :description="t('apiTokens.permissionsHint')" name="scopes"><div class="grid gap-4"><fieldset v-for="group in groups" :key="group.group" class="grid gap-2 sm:grid-cols-2"><legend class="text-sm text-muted mb-2">{{ group.label }}</legend><UCheckbox v-for="scope in group.scopes" :id="`api-token-scope-${scope.replace(':', '-')}`" :key="scope" :value="scope" :model-value="draft.scopes.includes(scope)" :label="t(`apiTokens.scopes.${scope}`)" :disabled="busy" @update:model-value="toggle(scope, $event === true)" /></fieldset></div></UFormField>
    </fieldset>
    <div><UButton type="submit" icon="i-ph-plus" :label="t('apiTokens.create')" :loading="busy" /></div>
  </UForm>
</template>
