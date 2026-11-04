<script setup lang="ts">
import { ref } from 'vue'
import { t } from '@/i18n'
defineProps<{ enabled: boolean; busy: boolean; error: string }>()
const emit = defineEmits<{ grant: [ip: string] }>()
const ip = ref('')
</script>

<template>
  <form class="grid gap-3" @submit.prevent="emit('grant', ip)">
    <UFormField :label="t('connections.ip')" :description="t('connections.grantHelp')" :error="error || undefined">
      <div class="flex flex-wrap gap-2">
        <UInput v-model="ip" name="ip" autocomplete="off" autocapitalize="off" spellcheck="false" class="min-w-0 flex-1" :disabled="!enabled || busy" :placeholder="t('connections.ipPlaceholder')" required maxlength="45" />
        <UButton type="submit" :label="t('connections.add')" :loading="busy" :disabled="!enabled || busy || !ip.trim()" />
      </div>
    </UFormField>
    <p v-if="!enabled" class="text-sm text-muted">{{ t('connections.grantUnavailable') }}</p>
  </form>
</template>
