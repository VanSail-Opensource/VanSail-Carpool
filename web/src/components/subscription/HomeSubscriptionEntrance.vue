<script setup lang="ts">
import { useRouter } from 'vue-router'
import { useI18n } from '@/i18n'

const { t } = useI18n()
const router = useRouter()
function openSubscription(): void { void router.push({ name: 'subscription' }).catch(() => undefined) }
</script>

<template>
  <section class="section-block subscription-entrance">
    <UButton type="button" color="neutral" variant="link" icon="i-ph-link-bold" :label="t('subscription.title')" data-haptic="navigate" @click="openSubscription" />
  </section>
</template>

<style scoped>
.subscription-entrance { display: grid; justify-items: center; }
.subscription-entrance :deep(button) { min-height: 44px; }
@media (min-width: 900px) { .subscription-entrance { display: none; } }
</style>
