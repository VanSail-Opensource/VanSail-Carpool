<script setup lang="ts">
import AdminBackupsPanel from '@/components/admin/AdminBackupsPanel.vue'
import { useI18n } from '@/i18n'
defineProps<{ visible: boolean }>()
const { t } = useI18n()
</script>
<template>
  <UApp>
    <main class="backup-recovery-fixture">
      <AdminBackupsPanel v-if="visible" />
      <p v-else data-test="retired-backup-workspace">{{ t('browser.sessionExpired') }}</p>
    </main>
  </UApp>
</template>
<style scoped>
.backup-recovery-fixture { max-width: 80rem; margin: auto; padding: 1rem; min-width: 0; }
</style>
