<script setup lang="ts">
import { computed, shallowRef } from 'vue'
import { useI18n } from '@/i18n'
import { useXNodeGlobalSettings } from './useXNodeGlobalSettings'

const emit = defineEmits<{ saved: [] }>()
const { t } = useI18n()
const { draft, loading, busy, error, canSave, load, save } = useXNodeGlobalSettings('automaticWhitelist')
const saved = shallowRef(false)
const valid = computed(() => Number.isInteger(draft.value?.automaticWhitelist?.durationHours) && (draft.value?.automaticWhitelist?.durationHours ?? 0) >= 1 && (draft.value?.automaticWhitelist?.durationHours ?? 0) <= 8760)
async function submit() {
  if (!valid.value || !canSave.value) return
  saved.value = false
  await save()
  if (!error.value) { saved.value = true; emit('saved') }
}
</script>

<template>
  <section class="grid gap-4">
    <p class="text-sm text-muted">{{ t('automaticLeases.settingsHint') }}</p>
    <UProgress v-if="loading" />
    <UAlert v-if="error" color="warning" :title="error" />
    <UButton v-if="!loading && !canSave && !busy" class="justify-self-start" variant="ghost" :label="t('common.tryAgain')" @click="load" />
    <form v-if="draft?.automaticWhitelist" class="grid gap-4 sm:grid-cols-[1fr_1fr_auto] sm:items-end" @submit.prevent="submit">
      <UFormField><USwitch v-model="draft.automaticWhitelist.enabled" :disabled="!canSave" :label="t('automaticLeases.enabled')" @update:model-value="saved = false" /></UFormField>
      <UFormField :label="t('automaticLeases.duration')"><UInput v-model.number="draft.automaticWhitelist.durationHours" type="number" min="1" max="8760" step="1" required :disabled="!canSave" class="w-full" @update:model-value="saved = false" /></UFormField>
      <UButton type="submit" :label="t('common.save')" :loading="busy" :disabled="!canSave || !valid" />
    </form>
    <p v-if="saved" role="status" class="text-sm text-muted">{{ t('automaticLeases.saved') }}</p>
  </section>
</template>
