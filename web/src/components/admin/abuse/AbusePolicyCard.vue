<script setup lang="ts">
import { reactive, watch } from 'vue'
import type { AbusePolicy } from '@/api/abuse'
import { useI18n } from '@/i18n'

const props = defineProps<{ policy: AbusePolicy; busy: boolean }>()
const emit = defineEmits<{ save: [value: AbusePolicy] }>()
type FormError = { name?: string; message: string }
type PolicyForm = Omit<AbusePolicy, 'outboundTags'> & { outboundTags: string }
const outboundTagPattern = /^[A-Za-z0-9][A-Za-z0-9._-]{0,119}$/
const { t } = useI18n()
const form = reactive<PolicyForm>(policyForm(props.policy))

watch(() => props.policy, value => Object.assign(form, policyForm(value)))

function policyForm(policy: AbusePolicy): PolicyForm {
  return { ...policy, outboundTags: policy.outboundTags.join(', ') }
}

function parsedOutboundTags(value: string): string[] {
  return value.split(',').map(tag => tag.trim())
}

function validOutboundTags(value: string): boolean {
  const tags = parsedOutboundTags(value)
  return tags.length > 0 && tags.length <= 25 && tags.every(tag => outboundTagPattern.test(tag)) && new Set(tags.map(tag => tag.toLowerCase())).size === tags.length
}

function validate(value: Partial<PolicyForm>): FormError[] {
  const errors: FormError[] = []
  const { outboundTags = '', globalLimit = -1, streakSeconds = 0, warningValidityDays = 0, warningCooldownMinutes = -1 } = value
  if (!validOutboundTags(outboundTags)) errors.push({ name: 'outboundTags', message: t('adminAbuse.invalidOutboundTags') })
  if (!Number.isInteger(globalLimit) || globalLimit < 0 || globalLimit > 100000) errors.push({ name: 'globalLimit', message: t('adminAbuse.invalidGlobalLimit') })
  if (!Number.isInteger(streakSeconds) || streakSeconds < 1 || streakSeconds > 1800) errors.push({ name: 'streakSeconds', message: t('adminAbuse.invalidStreakSeconds') })
  if (!Number.isInteger(warningValidityDays) || warningValidityDays < 1 || warningValidityDays > 365) errors.push({ name: 'warningValidityDays', message: t('adminAbuse.invalidValidity') })
  if (!Number.isInteger(warningCooldownMinutes) || warningCooldownMinutes < 0 || warningCooldownMinutes > 525600) errors.push({ name: 'warningCooldownMinutes', message: t('adminAbuse.invalidCooldown') })
  return errors
}

function save(): void {
  emit('save', { ...form, outboundTags: parsedOutboundTags(form.outboundTags) })
}
</script>

<template>
  <section class="card policy-card">
    <div>
      <p class="eyebrow">{{ t('adminAbuse.policyEyebrow') }}</p>
      <h3>{{ t('adminAbuse.policyTitle') }}</h3>
    </div>
    <UForm :state="form" :validate="validate" @submit="save">
      <UFormField name="outboundTags" :label="t('adminAbuse.outboundTags')" :description="t('adminAbuse.outboundTagsCopy')">
        <UInput v-model="form.outboundTags" data-test="outbound-tags" :maxlength="3048" autocomplete="off" />
      </UFormField>
      <UFormField name="globalEnabled" :label="t('adminAbuse.globalEnabled')">
        <USwitch v-model="form.globalEnabled" />
      </UFormField>
      <UFormField name="globalLimit" :label="t('adminAbuse.globalLimit')">
        <UInputNumber v-model="form.globalLimit" :min="0" :max="100000" :step="1" :disable-wheel-change="true" />
      </UFormField>
      <UFormField name="streakSeconds" :label="t('adminAbuse.streakSeconds')" :description="t('adminAbuse.streakSecondsCopy')">
        <UInputNumber v-model="form.streakSeconds" data-test="streak-seconds" :min="1" :max="1800" :step="1" :disable-wheel-change="true" />
      </UFormField>
      <UFormField name="warningValidityDays" :label="t('adminAbuse.validity')">
        <UInputNumber v-model="form.warningValidityDays" :min="1" :max="365" :step="1" :disable-wheel-change="true" />
      </UFormField>
      <UFormField name="warningCooldownMinutes" :label="t('adminAbuse.warningCooldown')">
        <UInputNumber v-model="form.warningCooldownMinutes" :min="0" :max="525600" :step="1" :disable-wheel-change="true" />
      </UFormField>
      <UButton type="submit" :loading="busy" :label="t('common.save')" />
    </UForm>
  </section>
</template>

<style scoped>
.card,
.card :deep(form) {
  display: grid;
  gap: 0.85rem;
}

.card {
  padding: 0;
  border: 0;
  background: transparent;
}
</style>
