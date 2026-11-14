<script setup lang="ts">
import { computed, shallowRef, watch } from 'vue'
import type { GroupMemberTag } from '@/api/preferences'
import InlineNotice from '@/components/common/InlineNotice.vue'

const props = defineProps<{ state: GroupMemberTag; saving: boolean; disabled?: boolean; error: string | null }>()
const emit = defineEmits<{ save: [tag: string] }>()
const draft = shallowRef('')
watch(() => props.state.tag, value => { draft.value = value }, { immediate: true })
const valid = computed(() => Array.from(draft.value.trim()).length <= 16)
const changed = computed(() => draft.value.trim() !== props.state.tag)
</script>

<template>
  <div v-if="state.groupJoined" class="tag-editor">
    <UFormField :label="$t('settings.tag.label')" :description="$t('settings.tag.hint')" :error="!valid ? $t('settings.tag.tooLong') : undefined">
      <div class="tag-editor__controls"><UInput v-model="draft" :disabled="!state.editable || saving || disabled" :aria-label="$t('settings.tag.label')" class="tag-editor__input" /><UButton :label="$t('common.save')" :disabled="!state.editable || !valid || !changed || saving || disabled" :loading="saving" @click="emit('save', draft.trim())" /></div>
    </UFormField>
    <InlineNotice v-if="!state.editable" tone="warning">{{ $t(`settings.tag.reasons.${state.reasonCode}`) }}</InlineNotice>
    <InlineNotice v-if="error" tone="warning">{{ error }}</InlineNotice>
  </div>
</template>

<style scoped>
.tag-editor { display: grid; gap: 0.75rem; padding-top: 1rem; }
.tag-editor__controls { display: flex; align-items: center; gap: 0.5rem; }
.tag-editor__input { min-width: 0; flex: 1; }
.tag-editor__controls :deep(button) { flex-shrink: 0; }
</style>
