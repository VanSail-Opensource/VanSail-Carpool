<script setup lang="ts">
import { computed } from 'vue'
import { en, zh_cn } from '@nuxt/ui/locale'
import { useI18n } from '@/i18n'
import AdminUserPM from '@/components/admin/users/AdminUserPM.vue'
import { useAdminUserProfile } from '@/components/admin/users/useAdminUserProfile'

const { locale, t } = useI18n()
const uiLocale = computed(() => locale.value === 'zh-CN' ? zh_cn : en)
const profile = useAdminUserProfile('ada')
</script>

<template>
  <UApp :locale="uiLocale">
    <main class="pm-recovery-audit">
      <div v-if="profile.error.value" data-test="pm-profile-error">
        <UAlert color="warning" :description="profile.error.value" />
        <UButton data-test="pm-profile-retry" :label="t('common.retry')" @click="profile.load(true)" />
      </div>
      <USkeleton v-if="profile.loading.value" class="h-32" />
      <AdminUserPM v-else-if="profile.detail.value" :conversation="profile.detail.value.pmConversation" @changed="profile.load(true)" />
    </main>
  </UApp>
</template>

<style scoped>
.pm-recovery-audit { max-width: 60rem; margin: auto; padding: 1.5rem; display: grid; gap: 1rem; }
</style>
