<script setup lang="ts">
import { computed, ref, shallowRef, watch } from 'vue'
import type { AbusePunishment } from '@/api/abuse'
import { useI18n } from '@/i18n'
import InlineNotice from '@/components/common/InlineNotice.vue'

const props = defineProps<{ punishments: AbusePunishment[]; busy: boolean; error?: string | null }>()
const emit = defineEmits<{ save: [value: AbusePunishment] }>()
type FormError = { name?: string; message: string }
const { t } = useI18n()
const ladder = ref(props.punishments.map(item => ({ ...item })))

const selectedAction = shallowRef('')
const selected = computed(() => ladder.value.find(item => item.action === selectedAction.value))

watch(() => props.punishments, value => { ladder.value = value.map(item => ({ ...item })) })

function validate(value: Partial<AbusePunishment>): FormError[] {
  const errors: FormError[] = []
  const { action = '', incidentThreshold = 0, durationMinutes = 0 } = value
  if (!Number.isInteger(incidentThreshold) || incidentThreshold < 1 || incidentThreshold > 100000) errors.push({ name: 'incidentThreshold', message: t('adminAbuse.invalidThreshold') })
  if ((action === 'ip_ban' || action === 'temporary_ban') && (!Number.isInteger(durationMinutes) || durationMinutes < 1 || durationMinutes > 525600)) errors.push({ name: 'durationMinutes', message: t('adminAbuse.invalidDuration') })
  return errors
}
</script>

<template>
  <section class="card ladder-card">
    <div>
      <p class="eyebrow">{{ t('adminAbuse.escalationEyebrow') }}</p>
      <h3>{{ t('adminAbuse.punishmentsTitle') }}</h3>
    </div>
    <UTable class="admin-table" :data="ladder" :columns="[{ id: 'action', header: t('adminAbuse.action') }, { accessorKey: 'incidentThreshold', header: t('adminAbuse.incidentThreshold') }, { accessorKey: 'durationMinutes', header: t('adminAbuse.duration') }, { id: 'status', header: t('adminWorkspace.status') }, { id: 'actions', header: t('adminWorkspace.actions') }]">
      <template #action-cell="{ row }">{{ t(`abuse.action.${row.original.action}`) }}</template>
      <template #status-cell="{ row }"><UBadge :color="row.original.enabled ? 'success' : 'neutral'" variant="subtle" :label="t(row.original.enabled ? 'common.enabled' : 'common.disabled')" /></template>
      <template #actions-cell="{ row }"><UButton color="neutral" variant="ghost" icon="i-ph-pencil-simple" :aria-label="t('adminWorkspace.details')" @click="selectedAction = row.original.action" /></template>
    </UTable>
    <USlideover :open="!!selected" :title="selected ? t(`abuse.action.${selected.action}`) : t('adminAbuse.punishmentsTitle')" :description="t('adminAbuse.escalationEyebrow')" :dismissible="!busy" @update:open="!$event && (selectedAction = '')">
      <template #body>
        <InlineNotice v-if="error" tone="warning">{{ error }}</InlineNotice>
        <UForm
          v-if="selected"
          :state="selected"
          :validate="validate"
          class="admin-editor-form"
          @submit="emit('save', { ...selected })"
        >
          <strong>{{ t(`abuse.action.${selected.action}`) }}</strong>
          <UFormField name="enabled" :label="t('adminAbuse.ruleEnabled')">
            <USwitch v-model="selected.enabled" />
          </UFormField>
          <UFormField name="incidentThreshold" :label="t('adminAbuse.incidentThreshold')">
            <UInputNumber v-model="selected.incidentThreshold" :min="1" :max="100000" :step="1" :disable-wheel-change="true" />
          </UFormField>
          <UFormField v-if="selected.action === 'ip_ban' || selected.action === 'temporary_ban'" name="durationMinutes" :label="t('adminAbuse.duration')">
            <UInputNumber v-model="selected.durationMinutes" :min="1" :max="525600" :step="1" :disable-wheel-change="true" />
          </UFormField>
          <UFormField v-if="selected.action === 'ip_ban'" name="allNodes" :label="t('adminAbuse.allNodes')">
            <USwitch v-model="selected.allNodes" />
          </UFormField>
          <UButton type="submit" size="sm" :loading="busy" :label="t('common.save')" />
        </UForm>
      </template>
    </USlideover>
  </section>
</template>

<style scoped>.card { display: grid; gap: 1rem; padding: 0; }</style>
