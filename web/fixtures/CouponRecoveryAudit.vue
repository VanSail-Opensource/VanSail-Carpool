<script setup lang="ts">
import AdminCouponsPanel from '@/components/admin/AdminCouponsPanel.vue'
import { useI18n } from '@/i18n'
defineProps<{ visible: boolean }>()
const { t } = useI18n()
</script>
<template>
  <UApp>
    <main class="coupon-recovery-fixture">
      <AdminCouponsPanel v-if="visible" />
      <p v-else data-test="retired-coupon-workspace">{{ t('browser.sessionExpired') }}</p>
    </main>
  </UApp>
</template>
<style scoped>
.coupon-recovery-fixture { max-width: 80rem; margin: auto; padding: 1rem; min-width: 0; }
</style>
