<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from '@/i18n'
import { useSessionStore } from '@/stores/session'
import { getTelegramWebApp } from '@/utils/telegram'

const session = useSessionStore()
const { t } = useI18n()
const telegramUser = computed(() => {
  const user = getTelegramWebApp()?.initDataUnsafe.user
  return user && String(user.id) === session.user?.telegramId ? user : undefined
})
const displayName = computed(() => session.user?.telegramUsername?.trim() || [session.user?.firstName, session.user?.lastName].filter(Boolean).join(' ').trim() || t('nav.memberFallback'))
const username = computed(() => telegramUser.value?.username?.trim() || session.user?.telegramUsername?.trim() || '')
const avatar = computed(() => {
  const photo = telegramUser.value?.photo_url
  return photo?.startsWith('https://') ? photo : undefined
})
</script>

<template>
  <div class="side-rail__member">
    <UAvatar :src="avatar" :alt="displayName" size="sm" referrerpolicy="no-referrer" />
    <div>
      <strong :title="t('nav.fullscreenGreeting', { name: displayName })">{{ t('nav.fullscreenGreeting', { name: displayName }) }}</strong>
      <span v-if="username" :title="`@${username}`">@{{ username }}</span>
    </div>
  </div>
</template>
