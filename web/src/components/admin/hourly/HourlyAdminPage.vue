<script setup lang="ts">
import { onMounted, shallowRef } from 'vue'
import { api } from '@/api/client'
import { hourlyApi, type HourlyPolicy, type HourlyPolicySnapshot, type HourlyOffer } from '@/api/hourly'
import type { SquadProduct } from '@/api/types'
import { ApiError } from '@/api/http'
import InlineNotice from '@/components/common/InlineNotice.vue'
import { localizedError, useI18n } from '@/i18n'
import AdminSectionState from '../AdminSectionState.vue'
import HourlyPolicyForm from './HourlyPolicyForm.vue'
import HourlyOfferForm from './HourlyOfferForm.vue'
const { t } = useI18n()
const policy = shallowRef<HourlyPolicySnapshot | null>(null)
const offers = shallowRef<HourlyOffer[]>([])
const squads = shallowRef<readonly SquadProduct[]>([])
const loading = shallowRef(true)
const busy = shallowRef(false)
const error = shallowRef<string | null>(null)
const actionError = shallowRef<string | null>(null)
const saved = shallowRef(false)
let loadVersion = 0
async function load(): Promise<void> {
  const version = ++loadVersion
  loading.value = true; error.value = null
  try {
    const [nextPolicy, nextOffers, nextSquads] = await Promise.all([hourlyApi.policy(), hourlyApi.offers(), api.getAdminResource<{ items: SquadProduct[] }>('squad-products')])
    if (version !== loadVersion) return
    policy.value = nextPolicy; offers.value = nextOffers
    // Hourly access is sold only for combo squads; pay-as-you-go (charge by
    // traffic) squads have their own per-GB offer and must not appear here.
    squads.value = nextSquads.items.filter(squad => squad.billingMode !== 'payg')
    actionError.value = null
  } catch (caught) { if (version === loadVersion) error.value = localizedError(caught, 'hourly.loadFailed') }
  finally { if (version === loadVersion) loading.value = false }
}
async function perform(action: () => Promise<void>): Promise<void> {
  if (busy.value) return
  busy.value = true; actionError.value = null; saved.value = false
  try { await action(); saved.value = true }
  catch (caught) { actionError.value = caught instanceof ApiError && caught.status === 409 ? t('hourly.conflict') : localizedError(caught, 'hourly.actionFailed') }
  finally { busy.value = false }
}
function savePolicy(next: HourlyPolicy): Promise<void> {
  return perform(async () => { policy.value = await hourlyApi.savePolicy(policy.value!.version, next) })
}
function saveOffer(groupId: string, revision: string, enabled: boolean, minor: string, requiredHours: number): Promise<void> {
  return perform(async () => {
    const next = await hourlyApi.saveOffer(groupId, revision, enabled, minor, requiredHours)
    offers.value = [...offers.value.filter(value => value.groupId !== groupId), next]
  })
}
onMounted(() => void load())
</script>

<template>
  <section class="admin-panel hourly-admin">
    <div class="admin-panel__heading"><div><h2>{{ t('hourly.admin.title') }}</h2><p>{{ t('hourly.admin.intro') }}</p></div><UButton color="neutral" variant="outline" :label="t('common.refresh')" :disabled="busy || loading" icon="i-ph-arrow-clockwise" @click="load" /></div>
    <AdminSectionState :loading="loading" :error="error" @retry="load">
      <div v-if="policy" class="hourly-admin__content">
        <InlineNotice v-if="actionError" tone="warning">{{ actionError }}</InlineNotice>
        <InlineNotice v-else-if="saved" tone="success">{{ t('hourly.admin.saved') }}</InlineNotice>
        <h3>{{ t('hourly.admin.policy') }}</h3>
        <HourlyPolicyForm :snapshot="policy" :busy="busy" @save="savePolicy" />
        <section class="hourly-admin__offers">
          <h3>{{ t('hourly.admin.offers') }}</h3><p>{{ t('hourly.admin.offersHint') }}</p>
          <p v-if="!squads.length">{{ t('hourly.admin.noSquads') }}</p>
          <HourlyOfferForm v-for="squad in squads" :key="squad.groupId" :squad="squad" :offer="offers.find(value => value.groupId === squad.groupId)" :activation-hours="policy.policy.activationHours" :busy="busy" @save="saveOffer" />
        </section>
      </div>
    </AdminSectionState>
  </section>
</template>

<style scoped>
.hourly-admin__content { display: grid; gap: 1.25rem; }
.hourly-admin h3 { margin: 0; font-size: 1rem; font-weight: 650; }
.hourly-admin__offers { margin-top: 1.5rem; }
.hourly-admin__offers > p { margin: .5rem 0 1rem; color: var(--text-muted); line-height: 1.6; font-size: .875rem; }
</style>
