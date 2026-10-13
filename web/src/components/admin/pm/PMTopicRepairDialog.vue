<script setup lang="ts">
import './modal.css'
import { computed, reactive, watch } from 'vue'
import { z } from 'zod'
import InlineNotice from '@/components/common/InlineNotice.vue'
import type { PMConversation } from '@/api/types'
import { useI18n } from '@/i18n'
import { useMotionPreferences } from '@/composables/useMotionPreferences'

const open = defineModel<boolean>('open', { required: true })
const props = defineProps<{ conversation: PMConversation | null; busy: boolean; error?: string | null }>()
const emit = defineEmits<{ confirm: [body: { topicId: string; profileMessageId?: string }] }>()
const { t } = useI18n()
const { reducedMotion } = useMotionPreferences()
const draft = reactive({ topicId: '', profileMessageId: '' })
const reference = (minimum: bigint) => z.string().refine(value => /^[1-9]\d{0,18}$/.test(value) && BigInt(value) >= minimum && BigInt(value) <= 9223372036854775807n)
const schema = z.object({ topicId: reference(2n), profileMessageId: z.union([z.literal(''), reference(1n)]) })
const valid = computed(() => schema.safeParse(draft).success)
watch(open, (next) => { if (next) { draft.topicId = props.conversation?.topicId ?? ''; draft.profileMessageId = props.conversation?.profileMessageId ?? '' } })
function submit(): void {
  if (!valid.value || props.busy) return
  emit('confirm', { topicId: draft.topicId, ...(draft.profileMessageId ? { profileMessageId: draft.profileMessageId } : {}) })
}
</script>

<template>
  <UModal v-model:open="open" :title="t('pm.repair')" :description="t('pm.repairHint')" :dismissible="!busy" :close="!busy" :ui="{ content: reducedMotion ? 'pm-modal--reduced' : '', overlay: reducedMotion ? 'pm-modal--reduced' : '' }">
    <template #body>
      <InlineNotice v-if="error" tone="warning">{{ error }}</InlineNotice>
      <form class="pm-repair" @submit.prevent="submit">
        <UFormField name="topicId" :label="t('pm.topicId')" :hint="t('pm.topicIdHint')"><UInput v-model="draft.topicId" name="topicId" class="w-full" inputmode="numeric" :maxlength="19" :disabled="busy" :aria-label="t('pm.topicId')" /></UFormField>
        <UFormField name="profileMessageId" :label="t('pm.profileId')" :hint="t('pm.profileIdHint')"><UInput v-model="draft.profileMessageId" name="profileMessageId" class="w-full" inputmode="numeric" :maxlength="19" :disabled="busy" :aria-label="t('pm.profileId')" /></UFormField>
      </form>
    </template>
    <template #footer><UButton color="neutral" variant="outline" :label="t('common.cancel')" :disabled="busy" @click="open = false" /><UButton color="neutral" :label="t('pm.verifyTopic')" :disabled="!valid || busy" :loading="busy" @click="submit" /></template>
  </UModal>
</template>

<style scoped>
.pm-repair { display: grid; gap: 1rem; }
</style>
