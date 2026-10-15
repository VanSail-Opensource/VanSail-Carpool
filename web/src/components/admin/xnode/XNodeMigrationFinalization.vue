<script setup lang="ts">
import { computed, ref } from 'vue'
import type { XNodeMigration } from '@/api/xnode'
import { useI18n } from '@/i18n'

const props = defineProps<{ state: XNodeMigration; busy: boolean }>()
const emit = defineEmits<{ stage: [reference: string]; finalize: [digest: string] }>()
const { t } = useI18n()
const reference = ref('')
const ready = computed(() => props.state.phase === 'active' && props.state.nodes.length > 0 && props.state.nodes.every(node => node.phase === 'native'))
</script>

<template>
  <section v-if="ready || state.phase === 'complete'" class="grid gap-3 min-w-0">
    <h4 class="font-medium">{{ t('xnodeMigration.finalTitle') }}</h4>
    <p v-if="state.phase === 'complete'" role="status" class="text-sm text-muted">{{ t('xnodeMigration.completed') }}</p>
    <template v-else>
      <p class="text-sm text-muted">{{ t('xnodeMigration.finalHint') }}</p>
      <UFormField :label="t('xnodeMigration.finalSourceBackup')">
        <UInput v-model="reference" :maxlength="512" :disabled="busy" class="w-full" />
      </UFormField>
      <div class="flex flex-wrap gap-2">
        <UButton :label="t('xnodeMigration.finalExport')" :disabled="busy || !reference.trim()" @click="emit('stage', reference.trim())" />
        <UButton v-if="state.finalDigest" :label="t('xnodeMigration.finalize')" color="neutral" variant="outline" :disabled="busy" @click="emit('finalize', state.finalDigest)" />
      </div>
      <p v-if="state.finalDigest" class="text-xs text-muted break-all">{{ t('xnodeMigration.finalSnapshot', { digest: state.finalDigest, reference: state.finalBackupReference }) }}</p>
    </template>
  </section>
</template>
