<script setup lang="ts">
import { computed, shallowRef } from 'vue'
import type { PMConversation } from '@/api/types'
import { pmApi } from '@/api/pm'
import SwitchField from '@/components/common/SwitchField.vue'
import OperationStatusNotice from '@/components/common/OperationStatusNotice.vue'
import StatusBadge from '@/components/common/StatusBadge.vue'
import { useDurableCommand } from '@/composables/useDurableCommand'
import { useI18n } from '@/i18n'
import PMDeliveryDialog from '../pm/PMDeliveryDialog.vue'
import PMTopicRepairDialog from '../pm/PMTopicRepairDialog.vue'
import { pmTopicLink } from '../pm/links'

const props = defineProps<{ conversation: PMConversation | null }>()
const emit = defineEmits<{ changed: [] }>()
const { t } = useI18n()
const deliveriesOpen = shallowRef(false)
const repairOpen = shallowRef(false)
const command = useDurableCommand({ errorKey: 'pm.actionFailed', onTerminal: () => emit('changed') })
const busy = computed(() => command.busy.value)
const statusMessage = computed(() => t(command.receipt.value?.status === 'succeeded' ? 'pm.updated'
  : command.receipt.value?.status === 'pending_review' ? 'pm.reviewHint'
    : command.receipt.value?.status === 'failed' ? 'pm.actionFailed' : 'pm.pending'))

function begin(): boolean {
  if (busy.value) return false
  if (command.blocksMutations.value) command.reset()
  return true
}

async function moderate(key: 'blocked' | 'muted', value: boolean): Promise<void> {
  const item = props.conversation
  if (!item || !begin()) return
  await command.execute(item.id, `${item.id}:${key}:${value}`,
    idempotencyKey => pmApi.moderate(item.id, { [key]: value }, idempotencyKey))
  emit('changed')
}

async function repairTopic(body: { topicId: string; profileMessageId?: string }): Promise<void> {
  const item = props.conversation
  if (!item || !begin()) return
  if (await command.execute(item.id, `${item.id}:${JSON.stringify(body)}`,
    key => pmApi.repair(item.id, body, key))) repairOpen.value = false
}

function openTopic(): void {
  const item = props.conversation
  if (!item) return
  const link = pmTopicLink(item.chatId, item.topicId)
  if (link) globalThis.open(link, '_blank', 'noopener,noreferrer')
}
</script>

<template>
  <section class="admin-profile-section user-pm">
    <div class="admin-profile-section__heading">
      <div><h3>{{ t('pm.title') }}</h3><p>{{ t('pm.userProfileHint') }}</p></div>
      <UButton v-if="conversation?.topicId" color="neutral" variant="ghost" icon="i-ph-arrow-square-out" :label="t('pm.openTopic')" @click="openTopic" />
    </div>
    <OperationStatusNotice v-if="conversation" :receipt="command.receipt.value" :error="command.error.value" :checking="command.checking.value" :message="statusMessage" @refresh="command.refresh" />
    <div v-if="conversation" class="user-pm__body">
      <div class="user-pm__state">
        <StatusBadge :tone="conversation.topicState === 'ready' ? 'success' : 'warning'" :label="t(`pm.topicState.${conversation.topicState}`)" />
        <small v-if="conversation.profileState === 'pending_review'">{{ t('pm.errors.PM_PROFILE_UNCERTAIN') }}</small>
      </div>
      <div class="user-pm__switches">
        <SwitchField :id="`user-pm-block-${conversation.id}`" :model-value="conversation.blocked" :label="t('pm.blocked')" :disabled="busy" @update:model-value="moderate('blocked', $event)" />
        <SwitchField :id="`user-pm-mute-${conversation.id}`" :model-value="conversation.muted" :label="t('pm.muted')" :help="t('pm.muteHint')" :disabled="busy" @update:model-value="moderate('muted', $event)" />
      </div>
      <div class="user-pm__actions">
        <UButton color="neutral" variant="outline" :label="t('pm.deliveries')" @click="deliveriesOpen = true" />
        <UButton v-if="conversation.topicState === 'pending_review' || conversation.profileState === 'pending_review'" color="neutral" variant="ghost" :label="t('pm.repair')" :disabled="busy" @click="repairOpen = true" />
      </div>
    </div>
    <p v-else class="user-pm__empty">{{ t('pm.noConversationForUser') }}</p>
    <PMDeliveryDialog v-if="conversation" v-model:open="deliveriesOpen" :conversation="conversation" />
    <PMTopicRepairDialog v-if="conversation" v-model:open="repairOpen" :conversation="conversation" :busy="busy" :error="command.error.value" @confirm="repairTopic" />
  </section>
</template>

<style scoped>
.user-pm { display: grid; gap: 0.9rem; }
.admin-profile-section__heading { display: flex; align-items: start; justify-content: space-between; gap: 0.75rem; }
.admin-profile-section__heading h3,.admin-profile-section__heading p { margin: 0; }
.admin-profile-section__heading p,.user-pm__state small,.user-pm__empty { color: var(--text-muted); font-size: 0.8rem; }
.user-pm__body { display: grid; gap: 0.9rem; }
.user-pm__state { display: flex; flex-wrap: wrap; align-items: center; gap: 0.5rem; }
.user-pm__switches { display: grid; gap: 0.65rem; }
.user-pm__actions { display: flex; flex-wrap: wrap; gap: 0.5rem; }
.user-pm__empty { margin: 0; }
</style>
