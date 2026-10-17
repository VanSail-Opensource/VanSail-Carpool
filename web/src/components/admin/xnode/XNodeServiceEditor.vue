<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from '@/i18n'
import type { NamedService, NamedServiceWrite, XNodeGroup } from '@/api/xnode'

const props = defineProps<{ item: NamedService | null; groups: XNodeGroup[]; busy: boolean; error?: string | null }>()
const emit = defineEmits<{ save: [value: NamedServiceWrite]; close: [] }>()
const { t } = useI18n()
const now = new Date()
const draft = ref<NamedServiceWrite>(props.item ? { name: props.item.identity.serviceName, access: globalThis.structuredClone(props.item.access) } : { name: '', access: { limitBytes: '0', strategy: 'NO_RESET', startedAt: now.toISOString(), expiresAt: '', groupIds: [], disabled: false } })
const expiry = computed({ get: () => draft.value.access.expiresAt ? new Date(draft.value.access.expiresAt).toLocaleString('sv-SE').replace(' ', 'T').slice(0, 16) : '', set: value => { draft.value.access.expiresAt = value ? new Date(value).toISOString() : '' } })
const strategies = computed(() => ['NO_RESET', 'DAY', 'WEEK', 'MONTH', 'MONTH_ROLLING'].map(value => ({ value, label: t(`namedServices.strategies.${value}`) })))
const groups = computed(() => props.groups.map(group => ({ value: group.id, label: group.name })))
const valid = computed(() => /^[A-Za-z0-9._-]{1,128}$/.test(draft.value.name) && /^\d+$/.test(draft.value.access.limitBytes) && BigInt(draft.value.access.limitBytes) <= 9223372036854775807n && draft.value.access.groupIds.length > 0 && Date.parse(draft.value.access.expiresAt) > Date.now())
</script>

<template>
  <UModal :open="true" :title="t(item ? 'namedServices.edit' : 'namedServices.add')" @update:open="!$event && emit('close')">
    <template #body>
      <UAlert v-if="error" color="warning" :title="error" />
      <form class="grid gap-4" @submit.prevent="valid && emit('save', draft)">
        <UFormField :label="t('common.name')"><UInput v-model="draft.name" required :disabled="!!item || busy" class="w-full" /></UFormField>
        <UFormField :label="t('namedServices.quota')"><UInput v-model="draft.access.limitBytes" inputmode="numeric" required :disabled="busy" class="w-full" /></UFormField>
        <UFormField :label="t('namedServices.reset')"><USelect v-model="draft.access.strategy" :items="strategies" :disabled="busy" class="w-full" /></UFormField>
        <UFormField :label="t('namedServices.expiry')"><UInput v-model="expiry" type="datetime-local" required :disabled="busy" class="w-full" /></UFormField>
        <UFormField :label="t('namedServices.groups')"><UCheckboxGroup v-model="draft.access.groupIds" :items="groups" :disabled="busy" /></UFormField>
        <USwitch v-model="draft.access.disabled" :label="t('namedServices.disabled')" :disabled="busy" />
        <UButton type="submit" :label="t('common.save')" :disabled="!valid" :loading="busy" class="justify-self-end" />
      </form>
    </template>
  </UModal>
</template>
