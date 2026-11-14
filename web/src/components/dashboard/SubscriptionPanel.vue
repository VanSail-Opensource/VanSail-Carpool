<script setup lang="ts">
import { shallowRef, watch } from 'vue'
import { useRouter } from 'vue-router'

import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import { useClipboard } from '@/composables/useClipboard'

const props = defineProps<{
  subscriptionUrl?: string | null
  revoking: boolean
  revokeBlocked: boolean
  openRevoke?: boolean
}>()

const emit = defineEmits<{ revoke: []; revokeRequestConsumed: [] }>()
const confirmOpen = shallowRef(false)
const { copied, copy, reset } = useClipboard()
const router = useRouter()

watch(() => props.subscriptionUrl, () => {
  reset()
  confirmOpen.value = false
}, { flush: 'sync' })
watch(() => props.revoking, (next, previous) => {
  if (previous && !next) confirmOpen.value = false
})
watch(() => props.openRevoke, (requested) => {
  if (!requested) return
  if (props.subscriptionUrl && !props.revokeBlocked) {
    confirmOpen.value = true
    return
  }
  emit('revokeRequestConsumed')
}, { immediate: true })
watch(confirmOpen, (open, wasOpen) => {
  if (!open && wasOpen && props.openRevoke) emit('revokeRequestConsumed')
})

function copyLink(): void {
  if (props.subscriptionUrl) void copy(props.subscriptionUrl)
}

function confirmRevoke(): void {
  if (confirmOpen.value && props.subscriptionUrl && !props.revokeBlocked && !props.revoking) emit('revoke')
}

function openConnections(): void {
  void router.push('/connections').catch(() => undefined)
}
</script>

<template>
  <section class="section-block home-subscription">
    <div class="section-heading">
      <h2>{{ $t('dashboard.subscriptionLink') }}</h2>
      <span class="feature-icon feature-icon--small"><UIcon name="i-ph-key" /></span>
    </div>
    <template v-if="subscriptionUrl">
      <p class="home-subscription__lead">{{ $t('dashboard.subscriptionHint') }}</p>
      <div class="home-subscription__value">
        <code>{{ subscriptionUrl }}</code>
        <UButton
          class="home-subscription__copy"
          color="neutral"
          variant="ghost"
          :icon="copied ? 'i-ph-check-bold' : 'i-ph-copy'"
          :aria-label="copied ? $t('common.copied') : $t('dashboard.copySubscription')"
          data-haptic="copy"
          @click="copyLink"
        />
      </div>
      <div class="home-subscription__actions">
        <UButton
          class="home-subscription__connections"
          color="neutral"
          variant="outline"
          icon="i-ph-devices"
          :label="$t('connections.open')"
          data-haptic="navigate"
          @click="openConnections"
        />
        <UButton
          class="home-subscription__revoke"
          color="error"
          variant="solid"
          icon="i-ph-trash"
          :disabled="revokeBlocked"
          :label="$t('common.revoke')"
          data-haptic="open"
          @click="confirmOpen = true"
        />
      </div>
    </template>
    <div v-else class="empty-inline">
      <div>
        <h3>{{ $t('dashboard.noActiveLink') }}</h3>
        <p>{{ $t('dashboard.linkAfterActivation') }}</p>
      </div>
    </div>

    <ConfirmDialog
      v-model:open="confirmOpen"
      :title="$t('dashboard.revokeTitle')"
      :description="$t('dashboard.revokeDescription')"
      :confirm-label="$t('dashboard.revokeLink')"
      :busy="revoking"
      :show-close="false"
      centered
      danger
      @confirm="confirmRevoke"
    />
  </section>
</template>

<style scoped>
@media (min-width: 900px) { .home-subscription__actions { display: none; } }
</style>
