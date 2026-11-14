<script setup lang="ts">
import { shallowRef } from 'vue'
import { t } from '@/i18n'
defineProps<{ enabled: boolean; busy: boolean; error: string }>()
const emit = defineEmits<{ grant: [ip: string] }>()
const ip = shallowRef('')
</script>

<template>
  <form class="connection-grant" @submit.prevent="emit('grant', ip)">
    <div class="connection-grant__heading"><span class="connection-grant__icon"><UIcon name="i-ph-shield-plus" aria-hidden="true" /></span><h2>{{ t('connections.grantTitle') }}</h2></div>
    <p class="connection-grant__help">{{ t('connections.grantHelp') }}</p>
    <UFormField :label="t('connections.ip')" :error="error || undefined">
      <UInput v-model="ip" name="ip" autocomplete="off" autocapitalize="off" spellcheck="false" class="w-full" :disabled="!enabled || busy" :placeholder="t('connections.ipPlaceholder')" required maxlength="45" />
    </UFormField>
    <UButton type="submit" icon="i-ph-plus" :label="t('connections.add')" :loading="busy" :disabled="!enabled || busy || !ip.trim()" block />
    <p v-if="!enabled" class="connection-grant__unavailable"><UIcon name="i-ph-lock-key" aria-hidden="true" />{{ t('connections.grantUnavailable') }}</p>
  </form>
</template>

<style scoped>
.connection-grant { display: grid; gap: .9rem; padding: 1rem; border: 1px solid var(--line); border-radius: var(--radius-panel); background: var(--surface); }
.connection-grant__heading { display: flex; align-items: center; gap: .65rem; }
.connection-grant__heading h2 { margin: 0; font-size: .9rem; font-weight: 600; }
.connection-grant__icon { display: inline-flex; align-items: center; justify-content: center; width: 2.25rem; height: 2.25rem; color: var(--accent); background: var(--accent-soft); border-radius: .75rem; font-size: 1.2rem; }
.connection-grant__help { margin: 0; font-size: .75rem; line-height: 1.6; color: var(--text-muted); }
.connection-grant__unavailable { display: flex; align-items: start; gap: .4rem; margin: 0; font-size: .72rem; line-height: 1.55; color: var(--text-muted); }
.connection-grant__unavailable > span { flex: 0 0 auto; margin-top: .2rem; }
</style>
