<script setup lang="ts">
import { computed, onMounted, shallowRef } from 'vue'
import { en, zh_cn } from '@nuxt/ui/locale'
import { localizedError, useI18n } from '@/i18n'
import type { PMConversation } from '@/api/types'
import AdminUserPM from '@/components/admin/users/AdminUserPM.vue'
const props = defineProps<{ loadConversation: () => Promise<PMConversation | null> }>()
const { locale, t } = useI18n()
const uiLocale = computed(() => locale.value === 'zh-CN' ? zh_cn : en)
const conversation = shallowRef<PMConversation | null>(null)
const error = shallowRef('')
async function reload(): Promise<void> {
  error.value = ''
  try { conversation.value = await props.loadConversation() }
  catch (caught) { error.value = localizedError(caught, 'pm.actionFailed') }
}
onMounted(() => void reload())
</script>
<template><UApp :locale="uiLocale"><main class="pm-audit"><template v-if="error"><UAlert color="error" :description="error" /><UButton :label="t('common.tryAgain')" @click="reload" /></template><AdminUserPM :conversation="conversation" @changed="reload" /></main></UApp></template>
<style scoped>.pm-audit { max-width: 60rem; margin: auto; padding: 1.5rem; }</style>
