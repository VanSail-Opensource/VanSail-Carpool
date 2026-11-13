<script setup lang="ts">
import { shallowRef } from 'vue'
import { api } from '@/api/client'
import type { Session } from '@/api/types'
import CaptchaGate from '@/components/session/CaptchaGate.vue'
import AdminSettingsPanel from '@/components/admin/AdminSettingsPanel.vue'

const verified = shallowRef<Session | null>(null)
const params = new globalThis.URLSearchParams(globalThis.location.search)
const challenge = { required: true, siteKey: params.has('missing') ? '' : '1x00000000000000000000AA', action: 'txc_first_entry' as const }
</script>

<template>
  <UApp>
    <AdminSettingsPanel v-if="params.has('admin')" />
    <CaptchaGate v-else-if="!verified" :challenge="challenge" @verified="verified = $event" />
    <main v-else class="auth-screen"><h1>{{ $t('onboarding.agreementTitle') }}</h1><UButton :label="$t('captcha.retry')" @click="api.getMe().then(() => verified = null)" /></main>
  </UApp>
</template>
