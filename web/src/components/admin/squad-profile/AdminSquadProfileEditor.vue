<script setup lang="ts">
import { computed, reactive, shallowRef, watch } from 'vue'

import type { SquadProfileWrite } from '@/api/types'
import SquadProfileSummary from '@/components/squad-profile/SquadProfileSummary.vue'
import { useI18n } from '@/i18n'
import { profileTypeOptions, type ProfileType } from '@/components/squad-profile/profile'
import BroadbandProfileFields from './BroadbandProfileFields.vue'
import ChinaOptimizedProfileFields from './ChinaOptimizedProfileFields.vue'
import InternationalNetworkProfileFields from './InternationalNetworkProfileFields.vue'
import { draftFromProfile, profileFromDraft, validationKey, type ProfileDraft } from './profileForm'

const props = withDefaults(defineProps<{
  showValidation?: boolean
}>(), { showValidation: false })
const profile = defineModel<SquadProfileWrite | null>('profile', { required: true })
const { t } = useI18n()
const draft = reactive<ProfileDraft>(draftFromProfile(profile.value))
const typeItems = computed(() => profileTypeOptions(t))
const isValid = computed(() => profileFromDraft(draft) !== null)
const validationMessage = computed(() => props.showValidation && !isValid.value ? t(validationKey(draft)) : '')
const profileSyncInProgress = shallowRef(false)

watch(() => profile.value, (value) => {
  if (profileSyncInProgress.value) {
    profileSyncInProgress.value = false
    return
  }
  Object.assign(draft, draftFromProfile(value))
}, { deep: true })
watch(draft, () => {
  const nextProfile = profileFromDraft(draft)
  if (nextProfile === null) {
    if (profile.value !== null) {
      profileSyncInProgress.value = true
      profile.value = null
    }
    return
  }
  profileSyncInProgress.value = true
  profile.value = nextProfile
}, { deep: true })

function changeType(value: string): void {
  if (!['broadband', 'china_optimized', 'international_network'].includes(value) || value === draft.type) return
  Object.assign(draft, draftFromProfile(null), { type: value as ProfileType })
}

function applyDraft(value: ProfileDraft): void {
  Object.assign(draft, value)
}
</script>

<template>
  <section class="squad-profile-editor">
    <UFormField name="squad-profile-type" :label="t('squadProfile.type')" required>
      <USelect :model-value="draft.type" :items="typeItems" value-key="value" @update:model-value="changeType" />
    </UFormField>
    <BroadbandProfileFields v-if="draft.type === 'broadband'" :draft="draft" @update:draft="applyDraft" />
    <ChinaOptimizedProfileFields v-else-if="draft.type === 'china_optimized'" :draft="draft" @update:draft="applyDraft" />
    <InternationalNetworkProfileFields v-else :draft="draft" @update:draft="applyDraft" />
    <UAlert v-if="validationMessage" color="warning" variant="soft" icon="i-ph-warning-circle">
      <template #description>{{ validationMessage }}</template>
    </UAlert>
    <div class="squad-profile-editor__preview">
      <span>{{ t('squadProfile.preview') }}</span>
      <SquadProfileSummary :profile="profile" compact />
    </div>
  </section>
</template>

<style scoped>
.squad-profile-editor { display: grid; gap: 0.85rem; }
.squad-profile-editor__preview { display: grid; gap: 0.45rem; padding: 0.75rem; border: 1px solid var(--line); border-radius: var(--radius-control); background: var(--surface); }
.squad-profile-editor__preview > span { color: var(--text-faint); font-size: 0.68rem; font-weight: 700; }
</style>
