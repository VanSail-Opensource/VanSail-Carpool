<script setup lang="ts">
import { computed } from 'vue'

import type { AdminUserDetail } from '@/api/adminOperations'
import StatusBadge from '@/components/common/StatusBadge.vue'
import { useI18n } from '@/i18n'
import { formatDateTime, formatMoney } from '@/utils/format'
import { adminUserName, hasPositiveBalance } from './adminUserFormat'

const props = defineProps<{ detail: AdminUserDetail }>()
const { t } = useI18n()

const identity = computed(() => props.detail.user.telegramUsername
  ? `@${props.detail.user.telegramUsername}`
  : props.detail.user.displayName || props.detail.user.telegramId)
const synchronizationTone = computed(() => props.detail.synchronization.status === 'synchronized'
  ? 'success' : props.detail.synchronization.status === 'failed' ? 'danger' : 'warning')
const synchronizationLabel = computed(() => props.detail.synchronization.status === 'not_provisioned'
  && hasPositiveBalance(props.detail.balance.minor)
  ? 'adminUserProfile.sync.payg_funded'
  : `adminUserProfile.sync.${props.detail.synchronization.status}`)
</script>

<template>
  <section class="profile-overview">
    <header class="profile-overview__identity">
      <UAvatar :text="adminUserName(detail).slice(0, 2).toUpperCase()" size="lg" />
      <div><h2>{{ adminUserName(detail) }}</h2><p>{{ identity }}</p></div>
      <StatusBadge :tone="synchronizationTone" :label="t(synchronizationLabel)" />
    </header>
    <dl class="profile-overview__facts">
      <div><dt>{{ t('adminUserProfile.balance') }}</dt><dd>{{ formatMoney(detail.balance) }}</dd></div>
      <div><dt>{{ t('adminUserProfile.telegramId') }}</dt><dd>{{ detail.user.telegramId }}</dd></div>
      <div><dt>{{ t('adminUserProfile.joined') }}</dt><dd>{{ formatDateTime(detail.user.createdAt) }}</dd></div>
    </dl>
    <div class="profile-overview__signals">
      <span><UIcon name="i-ph-arrows-clockwise" />{{ t('adminUserProfile.trafficResetAutomation') }} <strong>{{ t(detail.autoTrafficResetEnabled ? 'adminUserProfile.enabled' : 'adminUserProfile.disabled') }}</strong></span>
      <span>{{ t('adminUserProfile.accountUpdated') }} <time>{{ formatDateTime(detail.user.updatedAt) }}</time></span>
    </div>
    <div class="profile-overview__subscription">
      <UIcon name="i-ph-stack" /><div v-if="detail.activeCombo"><strong>{{ detail.activeCombo.comboName }}</strong><p>{{ t('adminUserProfile.validity', { from: formatDateTime(detail.activeCombo.validFrom), to: formatDateTime(detail.activeCombo.validUntil) }) }}</p></div><div v-else><strong>{{ t('adminUserProfile.noActiveCombo') }}</strong><p>{{ t('adminUserProfile.noActiveComboHint') }}</p></div>
    </div>
    <UAlert v-if="detail.synchronization.lastError" color="warning" variant="soft" icon="i-ph-warning-circle" :description="detail.synchronization.lastError" />
  </section>
</template>
<style scoped>
.profile-overview { display: grid; gap: 1.25rem; padding-bottom: 1.5rem; }
.profile-overview__identity { display: flex; gap: .85rem; align-items: center; flex-wrap: wrap; }
.profile-overview__identity > div { flex: 1; min-width: 0; }
.profile-overview h2 { margin: 0; font-size: 1.35rem; }
.profile-overview p { margin: .2rem 0 0; font-size: .8rem; color: var(--text-muted); }
.profile-overview__facts { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 1rem; margin: 0; padding: 1rem 0; border-block: 1px solid var(--line); }
.profile-overview__facts dt { color: var(--text-muted); font-size: .75rem; }
.profile-overview__facts dd { margin: .25rem 0 0; font-weight: 600; font-size: .95rem; overflow-wrap: anywhere; }
.profile-overview__signals { display: flex; flex-wrap: wrap; gap: .5rem 1.5rem; color: var(--text-muted); font-size: .75rem; }
.profile-overview__signals span { display: flex; align-items: center; gap: .5rem; }
.profile-overview__signals strong { color: var(--text); }
.profile-overview__subscription { display: flex; gap: .75rem; align-items: flex-start; }
.profile-overview__subscription > span { margin-top: .2rem; color: var(--accent); }
@media (min-width: 1100px) { .profile-overview__facts { grid-template-columns: repeat(4, minmax(0, 1fr)); } }
</style>
