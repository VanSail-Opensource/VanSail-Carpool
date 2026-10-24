<script setup lang="ts">
import { computed, shallowRef } from 'vue'
import { useRouter } from 'vue-router'

import type { Purchase } from '@/api/types'
import { api } from '@/api/client'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import InlineNotice from '@/components/common/InlineNotice.vue'
import StatusBadge from '@/components/common/StatusBadge.vue'
import RolloverFlipCard from './RolloverFlipCard.vue'
import PurchaseActions from './PurchaseActions.vue'
import { formatDate } from '@/utils/format'
import { localizedError } from '@/i18n'
import { notifyHaptic } from '@/utils/telegram'
import { usePurchaseValidity } from '@/composables/usePurchaseValidity'

const props = defineProps<{
  active?: Purchase | null
  queued?: Purchase | null
  squadNames?: readonly string[]
}>()
const emit = defineEmits<{ queuedCancelled: []; autoRenewalChanged: [] }>()
const validity = usePurchaseValidity(() => [props.active, props.queued].filter(item => item != null))
const currentActive = computed(() => validity.current(props.active) ? props.active : null)
const currentQueued = computed(() => [props.queued, props.active].find(item => item && validity.status(item) === 'queued'))

const router = useRouter()
const queuedCancelOpen = shallowRef(false)
const queuedCancelBusy = shallowRef(false)
const queuedCancelError = shallowRef<string | null>(null)

function openQueuedCancellation(): void {
  queuedCancelError.value = null
  queuedCancelOpen.value = true
}
async function cancelQueued(): Promise<void> {
  if (!currentQueued.value || queuedCancelBusy.value) return
  queuedCancelBusy.value = true
  queuedCancelError.value = null
  try {
    await api.cancelQueuedPurchase(currentQueued.value.id)
    queuedCancelOpen.value = false
    notifyHaptic('success')
    emit('queuedCancelled')
  } catch (caught) {
    queuedCancelError.value = localizedError(caught, 'errors.purchaseCancelFailed')
    notifyHaptic('error')
  } finally {
    queuedCancelBusy.value = false
  }
}

function goToCatalog(): void {
  void router.push('/catalog')
}

</script>

<template>
  <section class="section-block home-ride">
    <div class="section-heading">
      <h2>{{ $t('dashboard.yourRide') }}</h2>
      <StatusBadge v-if="currentActive" tone="success" :label="$t('common.active')" />
    </div>

    <div v-if="currentActive || currentQueued" class="home-ride__content">
      <RolloverFlipCard v-if="currentActive" :active="currentActive" :squad-names="squadNames" />
      <div v-if="currentQueued" class="home-ride__queued">
        <div class="home-ride__queued-content">
          <span>
            <StatusBadge tone="warning" :label="$t('common.queued')" />
            {{ $t('dashboard.queuedStarts', { name: currentQueued.comboName, date: formatDate(currentQueued.validFrom) }) }}
          </span>
          <UButton
            color="error"
            variant="ghost"
            size="sm"
            icon="i-ph-x-circle"
            :label="$t('home.cancelQueued')"
            data-haptic="open"
            @click="openQueuedCancellation"
          />
          <InlineNotice v-if="queuedCancelError" tone="warning">{{ queuedCancelError }}</InlineNotice>
        </div>
      </div>
      <PurchaseActions v-if="currentActive" :purchase="currentActive" @changed="emit('autoRenewalChanged')" />
    </div>

    <div v-else class="empty-inline">
      <div>
        <h3>{{ $t('dashboard.noActiveCombo') }}</h3>
        <p>{{ $t('dashboard.choosePlan') }}</p>
      </div>
      <UButton
        class="home-ride__catalog"
        color="neutral"
        variant="outline"
        trailing-icon="i-ph-arrow-right"
        :label="$t('catalog.viewCombos')"
        data-haptic="navigate"
        @click="goToCatalog"
      />
    </div>
  </section>
  <ConfirmDialog
    v-model:open="queuedCancelOpen"
    :title="$t('home.cancelQueuedTitle')"
    :description="$t('home.cancelQueuedDescription')"
    :confirm-label="$t('home.cancelQueuedConfirm')"
    :busy="queuedCancelBusy"
    danger
    @confirm="cancelQueued"
  />
</template>

<style scoped>
.home-ride__queued-content {
  display: grid;
  gap: 0.55rem;
  min-width: 0;
}

.home-ride__queued-content > span {
  min-width: 0;
}
</style>
