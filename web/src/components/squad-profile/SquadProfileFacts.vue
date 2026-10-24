<script setup lang="ts">
import { computed } from 'vue'

import type { SquadProfile } from '@/api/types'
import { useI18n } from '@/i18n'
import { countryName } from './profile'
import CarrierLogo from './CarrierLogo.vue'

const props = withDefaults(defineProps<{
  profile: SquadProfile | null
  presentation?: 'default' | 'member'
}>(), { presentation: 'default' })

const { locale, t } = useI18n()
const portText = computed(() => {
  if (!props.profile || !('portMbps' in props.profile)) return ''
  return props.profile.portMbps === null ? t('squadProfile.unlimited') : `${props.profile.portMbps} ${t('squadProfile.mbps')}`
})
const countryText = computed(() => {
  if (!props.profile || !('countryCode' in props.profile)) return ''
  return countryName(props.profile.countryCode, locale.value)
})
</script>

<template>
  <div v-if="profile" class="squad-profile-facts" :class="{ 'squad-profile-facts--member': presentation === 'member' }">
    <span v-if="profile.type === 'broadband'"><UIcon name="i-ph-broadcast" />{{ profile.isp }}</span>
    <span v-if="profile.type === 'broadband' && presentation !== 'member'"><UIcon name="i-ph-gauge" />{{ profile.portMbps }} {{ t('squadProfile.mbps') }}</span>
    <span v-if="profile.type === 'broadband'"><UIcon name="i-ph-arrows-clockwise" />{{ profile.dynamic ? t('squadProfile.dynamic') : t('squadProfile.static') }}</span>
    <span v-if="profile.type === 'broadband'"><UIcon name="i-ph-map-pin" />{{ profile.location }}</span>
    <template v-else-if="profile.type === 'china_optimized'">
      <span class="squad-profile-facts__carrier-route"><CarrierLogo carrier="telecom" :label="t('squadProfile.carriers.chinaTelecom')" />{{ profile.ct }}</span>
      <span class="squad-profile-facts__carrier-route"><CarrierLogo carrier="unicom" :label="t('squadProfile.carriers.chinaUnicom')" />{{ profile.cu }}</span>
      <span class="squad-profile-facts__carrier-route"><CarrierLogo carrier="mobile" :label="t('squadProfile.carriers.chinaMobile')" />{{ profile.cm }}</span>
      <span v-if="presentation !== 'member'"><UIcon name="i-ph-gauge" />{{ portText }}</span>
      <span><UIcon name="i-ph-map-pin" />{{ countryText }}</span>
    </template>
    <template v-else-if="profile.type === 'international_network'">
      <span v-if="presentation !== 'member'"><UIcon name="i-ph-gauge" />{{ portText }}</span>
      <span><UIcon name="i-ph-map-pin" />{{ countryText }}</span>
      <span><UIcon name="i-ph-broadcast" />{{ profile.upstreamCarriers.join(', ') }}</span>
    </template>
  </div>
</template>

<style scoped>
.squad-profile-facts { min-width: 0; display: flex; flex-wrap: wrap; gap: 0.35rem 0.7rem; color: var(--text-faint); font-size: 0.68rem; line-height: 1.45; }
.squad-profile-facts > span { display: inline-flex; align-items: center; gap: 0.25rem; min-width: 0; overflow-wrap: anywhere; }
.squad-profile-facts :deep(svg), .squad-profile-facts :deep(.iconify) { flex: 0 0 auto; }
.squad-profile-facts__carrier-route { font-weight: 600; }
.squad-profile-facts--member { gap: 0.35rem; }
.squad-profile-facts--member > span { max-width: 100%; padding: 0.27rem 0.38rem; border: 1px solid var(--squad-profile-tone-line); border-radius: 7px; color: var(--text-muted); background: color-mix(in srgb, var(--squad-profile-tone-soft) 70%, var(--surface)); }
</style>
