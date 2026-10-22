<script setup lang="ts">
import { onMounted, shallowRef } from 'vue'
import { firewallApi, type FirewallQuote, type FirewallGrant } from '@/api/firewall'
import { localizedError, useI18n } from '@/i18n'
import { createUuid } from '@/utils/browserCompatibility'
import { ApiError } from '@/api/http'
const { t } = useI18n()
const quote = shallowRef<FirewallQuote | null>(null)
const grant = shallowRef<FirewallGrant | null>(null)
const ip = shallowRef('')
const loading = shallowRef(true)
const busy = shallowRef(false)
const error = shallowRef<string | null>(null)
let key = createUuid()
let attemptedIP = ''
async function load(): Promise<void> {
  loading.value = true; error.value = null; quote.value = null
  try { quote.value = await firewallApi.quote() }
  catch (caught) { error.value = localizedError(caught, 'firewall.loadFailed') }
  finally { loading.value = false }
}
async function purchase(): Promise<void> {
  if (!quote.value?.enabled || busy.value || !ip.value.trim()) return
  if (attemptedIP !== ip.value.trim()) { attemptedIP = ip.value.trim(); key = createUuid() }
  busy.value = true; error.value = null
  try { grant.value = await firewallApi.purchase(attemptedIP, quote.value, key) }
  catch (caught) { error.value = caught instanceof ApiError && caught.code === 'XNODE_INPUT_INVALID' ? t('firewall.invalidIP') : caught instanceof ApiError && caught.code === 'XNODE_CONFLICT' ? t('firewall.conflict') : localizedError(caught, 'firewall.saveFailed') }
  finally { busy.value = false }
}
onMounted(() => void load())
</script>

<template>
  <UProgress v-if="loading" />
  <section v-else-if="!quote?.whitelistMode" class="grid gap-3 py-4">
    <div class="flex items-center justify-between gap-3"><h2 class="text-base font-semibold">{{ t('firewall.unbanTitle') }}</h2><UButton icon="i-ph-arrow-clockwise" color="neutral" variant="ghost" :loading="loading" :disabled="busy" :aria-label="t('common.refresh')" @click="load" /></div>
    <UAlert v-if="error" color="warning" :title="error" />
    <UAlert v-if="grant" color="success" :title="t('firewall.granted', { ip: grant.ip, until: new Date(grant.until).toLocaleString() })" :description="t('firewall.deliveryPending')" />
    <p v-if="quote && !quote.enabled" class="text-sm text-muted">{{ t('firewall.unavailable') }}</p>
    <form v-else-if="quote?.enabled" class="grid gap-3" @submit.prevent="purchase">
      <p class="text-sm text-muted">{{ t('firewall.exemptionHint') }}</p>
      <UFormField :label="t('firewall.ip')"><UInput v-model="ip" required :disabled="busy" autocomplete="off" class="w-full font-mono" /></UFormField>
      <UButton type="submit" icon="i-ph-shield-check" :loading="busy" :disabled="!ip.trim()" :label="t('firewall.purchase', { price: quote.price, hours: quote.hours })" class="justify-center" />
    </form>
  </section>
</template>
