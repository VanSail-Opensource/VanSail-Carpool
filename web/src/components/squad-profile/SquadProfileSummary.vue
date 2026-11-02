<script setup lang="ts">
import { computed } from 'vue'

import type { SquadProfile } from '@/api/types'
import { useI18n } from '@/i18n'
import { profileTypeMeta } from './profile'
import SquadProfileFacts from './SquadProfileFacts.vue'

const props = withDefaults(defineProps<{
  name?: string
  profile: SquadProfile | null
  compact?: boolean
  showFacts?: boolean
  presentation?: 'default' | 'member'
}>(), { name: '', compact: false, showFacts: true, presentation: 'default' })

defineSlots<{ facts?: () => unknown, namePrefix?: () => unknown, nameTags?: () => unknown, headingMeta?: () => unknown }>()

const { t } = useI18n()
const typeMeta = computed(() => props.profile ? profileTypeMeta(props.profile.type) : null)
const typeClass = computed(() => props.profile ? `squad-profile-summary--${props.profile.type}` : '')
const portText = computed(() => {
  if (!props.profile || !('portMbps' in props.profile)) return ''
  return props.profile.portMbps === null ? t('squadProfile.unlimited') : `${props.profile.portMbps} ${t('squadProfile.mbps')}`
})
const memberProfileText = computed(() => {
  const label = typeMeta.value ? t(typeMeta.value.labelKey) : t('squadProfile.unconfigured')
  return portText.value ? `${label} ${portText.value}` : label
})
</script>

<template>
  <div
    class="squad-profile-summary"
    :class="[typeClass, { 'squad-profile-summary--compact': compact, 'squad-profile-summary--member': presentation === 'member' }]"
  >
    <div class="squad-profile-summary__heading">
      <UIcon v-if="presentation !== 'member'" :name="typeMeta?.icon ?? 'i-ph-info'" aria-hidden="true" />
      <div class="squad-profile-summary__heading-copy">
        <span v-if="presentation === 'member' && name" class="squad-profile-summary__name-row" :class="{ 'squad-profile-summary__name-row--with-tags': $slots.nameTags }">
          <span class="squad-profile-summary__name-copy">
            <slot v-if="$slots.namePrefix" name="namePrefix" />
            <strong>{{ name }}</strong>
          </span>
          <span v-if="$slots.nameTags" class="squad-profile-summary__name-tags">
            <slot v-if="$slots.nameTags" name="nameTags" />
          </span>
          <slot name="headingMeta" />
        </span>
        <strong v-else>{{ typeMeta ? t(typeMeta.labelKey) : t('squadProfile.unconfigured') }}</strong>
        <span v-if="presentation === 'member' && name">{{ memberProfileText }}</span>
      </div>
    </div>
    <div v-if="showFacts && (profile || $slots.facts)" class="squad-profile-summary__facts">
      <SquadProfileFacts :profile="profile" :presentation="presentation" />
      <slot name="facts" />
    </div>
  </div>
</template>

<style scoped>
.squad-profile-summary { display: grid; gap: 0.55rem; min-width: 0; }
.squad-profile-summary__heading { display: flex; align-items: center; gap: 0.4rem; color: var(--text-muted); font-size: 0.72rem; }
.squad-profile-summary__heading-copy { display: grid; min-width: 0; gap: 0.08rem; }
.squad-profile-summary__name-row { min-width: 0; display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 0.35rem 0.5rem; }
.squad-profile-summary__name-copy { min-width: 0; display: flex; flex: 1 1 10rem; align-items: center; gap: 0.35rem; }
.squad-profile-summary__name-copy strong { min-width: 0; flex: 1 1 auto; }
.squad-profile-summary__name-tags { min-width: 0; display: inline-flex; flex: 0 1 auto; flex-wrap: wrap; align-items: center; gap: 0.35rem; }
.squad-profile-summary__heading-copy strong { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.squad-profile-summary__facts { min-width: 0; }
.squad-profile-summary--compact { gap: 0.35rem; }
.squad-profile-summary--member { --squad-profile-tone: var(--accent); --squad-profile-tone-soft: color-mix(in srgb, var(--accent) 11%, var(--surface)); --squad-profile-tone-line: color-mix(in srgb, var(--accent) 32%, var(--line)); gap: 0.6rem; }
.squad-profile-summary--member.squad-profile-summary--china_optimized { --squad-profile-tone: var(--warning); --squad-profile-tone-soft: color-mix(in srgb, var(--warning) 11%, var(--surface)); --squad-profile-tone-line: color-mix(in srgb, var(--warning) 32%, var(--line)); }
.squad-profile-summary--member.squad-profile-summary--international_network { --squad-profile-tone: #9ebddd; --squad-profile-tone-soft: color-mix(in srgb, #9ebddd 11%, var(--surface)); --squad-profile-tone-line: color-mix(in srgb, #9ebddd 32%, var(--line)); }
.squad-profile-summary--member .squad-profile-summary__heading { align-items: flex-start; color: var(--text); }
.squad-profile-summary--member .squad-profile-summary__heading-copy { flex: 1 1 auto; }
.squad-profile-summary--member .squad-profile-summary__heading-copy strong { overflow: visible; overflow-wrap: anywhere; text-overflow: clip; white-space: normal; }
.squad-profile-summary--member .squad-profile-summary__name-row--with-tags { flex-wrap: nowrap; justify-content: flex-start; }
.squad-profile-summary--member .squad-profile-summary__name-row--with-tags .squad-profile-summary__name-copy { flex: 0 1 auto; }
.squad-profile-summary--member .squad-profile-summary__name-row--with-tags .squad-profile-summary__name-copy strong { overflow: hidden; overflow-wrap: normal; text-overflow: ellipsis; white-space: nowrap; }
.squad-profile-summary--member .squad-profile-summary__name-row--with-tags .squad-profile-summary__name-tags { flex: 0 0 auto; flex-wrap: nowrap; }
.squad-profile-summary--member .squad-profile-summary__heading-copy span { color: var(--squad-profile-tone); font-size: 0.66rem; font-weight: 700; }
.squad-profile-summary--member.squad-profile-summary--compact { gap: 0.45rem; }
</style>
