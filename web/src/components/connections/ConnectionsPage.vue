<script setup lang="ts">
import { computed, shallowRef, watch } from 'vue'

import type { IPBlock } from '@/api/types'
import InlineNotice from '@/components/common/InlineNotice.vue'
import { useConnectionBlocks } from '@/composables/useConnectionBlocks'
import { useConnectionScan } from '@/composables/useConnectionScan'
import { t } from '@/i18n'
import ConnectionBlockDialog from './ConnectionBlockDialog.vue'
import FirewallExemption from './FirewallExemption.vue'
import ConnectionBlockList from './ConnectionBlockList.vue'
import ConnectionNodeList from './ConnectionNodeList.vue'
import ConnectionScanStatus from './ConnectionScanStatus.vue'
import ConnectionUnblockDialog from './ConnectionUnblockDialog.vue'
import type { ConnectionTarget } from './types'

const scan = useConnectionScan()
const blocks = useConnectionBlocks()
const selectedTarget = shallowRef<ConnectionTarget | null>(null)
const selectedBlock = shallowRef<IPBlock | null>(null)
const blockDialogOpen = shallowRef(false)
const unblockDialogOpen = shallowRef(false)
let refreshedOperationId = ''

const hasConnections = computed(() => scan.nodes.value.some((node) => node.ips.length > 0))
const operationTone = computed(() => blocks.receipt.value?.status === 'succeeded' ? 'success' : 'warning')
const operationMessage = computed(() => t(`connections.${blocks.action.value}Operation.${blocks.receipt.value?.status ?? 'queued'}`))

function selectTarget(target: ConnectionTarget): void {
  if (blocks.mutationActive.value) return
  blocks.resetOperation('block')
  selectedTarget.value = target
  blockDialogOpen.value = true
}

function selectBlock(block: IPBlock): void {
  if (blocks.mutationActive.value) return
  blocks.resetOperation('unblock')
  selectedBlock.value = block
  unblockDialogOpen.value = true
}

async function confirmBlock(): Promise<void> {
  if (selectedTarget.value) await blocks.block(selectedTarget.value.connection.handle)
}

async function confirmUnblock(): Promise<void> {
  if (selectedBlock.value) await blocks.unblock(selectedBlock.value.id)
}

async function refreshAll(): Promise<void> {
  await Promise.all([scan.restart(), blocks.load()])
}

watch(() => blocks.receipt.value, (receipt) => {
  if (!receipt || !blocks.terminal.value || receipt.id === refreshedOperationId) return
  refreshedOperationId = receipt.id
  void refreshAll()
})
</script>

<template>
  <div class="page page--connections">
    <header class="connections-heading">
      <div><span class="eyebrow">{{ $t('connections.eyebrow') }}</span><h1>{{ $t('connections.title') }}</h1><p>{{ $t('connections.subtitle') }}</p></div>
      <UButton color="neutral" variant="outline" square icon="i-ph-arrow-clockwise" :loading="scan.loading.value || scan.polling.value || blocks.loading.value" :disabled="blocks.mutationActive.value" :aria-label="$t('connections.scanAgain')" data-haptic="refresh" @click="refreshAll" />
    </header>

    <InlineNotice v-if="blocks.receipt.value && !blockDialogOpen && !unblockDialogOpen" :tone="operationTone" :title="$t(`operations.status.${blocks.receipt.value.status}`)">{{ operationMessage }}</InlineNotice>
    <FirewallExemption />
    <ConnectionBlockList v-if="scan.completed.value" :items="blocks.items.value" :loading="blocks.loading.value" :error="blocks.loadError.value" :disabled="blocks.mutationActive.value" @refresh="blocks.load" @unblock="selectBlock" />

    <ConnectionScanStatus v-if="scan.loading.value || scan.polling.value" :starting="scan.loading.value" :progress-percent="scan.progressPercent.value" />
    <section v-else-if="scan.error.value || scan.failed.value" class="error-state error-state--compact">
      <UIcon name="i-ph-warning-circle" class="connections-state-icon" aria-hidden="true" />
      <h2>{{ $t('connections.scanUnavailable') }}</h2><p>{{ scan.error.value ?? $t('connections.scanFailed') }}</p>
      <UButton icon="i-ph-arrow-clockwise" :label="$t('common.tryAgain')" data-haptic="retry" @click="scan.retry()" />
    </section>
    <section v-else-if="scan.completed.value && !hasConnections" class="connections-empty">
      <UIcon name="i-ph-plugs" class="connections-state-icon" aria-hidden="true" />
      <h2>{{ $t('connections.emptyTitle') }}</h2><p>{{ $t('connections.emptyDescription') }}</p>
      <UButton color="neutral" variant="outline" icon="i-ph-arrow-clockwise" :label="$t('connections.scanAgain')" data-haptic="refresh" @click="scan.restart()" />
    </section>
    <ConnectionNodeList v-else-if="scan.completed.value" :nodes="scan.nodes.value" :disabled="blocks.mutationActive.value" @block="selectTarget" />

    <ConnectionBlockDialog v-model:open="blockDialogOpen" :target="selectedTarget" :receipt="blocks.action.value === 'block' ? blocks.receipt.value : null" :busy="blocks.busyAction.value?.startsWith('block:')" :checking="blocks.checking.value" :error="blocks.action.value === 'block' ? blocks.error.value : null" @confirm="confirmBlock" @refresh="blocks.refreshOperation" />
    <ConnectionUnblockDialog v-model:open="unblockDialogOpen" :block="selectedBlock" :receipt="blocks.action.value === 'unblock' ? blocks.receipt.value : null" :busy="blocks.busyAction.value?.startsWith('unblock:')" :checking="blocks.checking.value" :error="blocks.action.value === 'unblock' ? blocks.error.value : null" @confirm="confirmUnblock" @refresh="blocks.refreshOperation" />
  </div>
</template>
