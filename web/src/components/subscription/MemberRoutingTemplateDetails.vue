<script setup lang="ts">
import { computed } from 'vue'
import type { RoutingView } from '@/api/xnodeMember'
import { useI18n } from '@/i18n'

const open = defineModel<boolean>('open', { required: true })
const props = defineProps<{ templates: RoutingView['templates']; excluded: string; status: string }>()
const { t } = useI18n()
const fields = ['domain', 'ip', 'port', 'network', 'protocol'] as const
const labels = { domain: 'domains', ip: 'ips', port: 'ports', network: 'networks', protocol: 'protocols' }
const rows = computed(() => props.templates.map(template => ({ ...template, fields: fields.flatMap(field => {
  const value = template.match[field]
  return value && value.length ? [{ label: t(`managedRouting.${labels[field]}`), value: Array.isArray(value) ? value.join('\n') : value }] : []
}) })))
</script>

<template>
  <UModal v-model:open="open" :title="t('managedRouting.templateDetails')" :description="t('managedRouting.templateDetailsHint')" :close="{ 'aria-label': t('common.close') }" scrollable>
    <template #body>
      <div class="grid gap-5 min-w-0">
        <p role="status" class="text-sm text-muted">{{ t(`managedRouting.routeState.${status}`) }}</p>
        <section v-for="template in rows" :key="template.id" class="grid gap-3 min-w-0">
          <h3 class="font-medium break-words">{{ template.name }}</h3>
          <p v-if="!template.fields.length" class="text-sm text-muted">{{ t('managedRouting.allTraffic') }}</p>
          <dl v-else class="grid gap-3">
            <div v-for="field in template.fields" :key="field.label" class="grid gap-1">
              <dt class="text-xs text-muted">{{ field.label }}</dt><dd class="whitespace-pre-wrap break-all font-mono text-sm">{{ field.value }}</dd>
            </div>
          </dl>
        </section>
        <p v-if="excluded" class="text-sm text-muted">{{ t('managedRouting.except', { conditions: excluded }) }}</p>
      </div>
    </template>
  </UModal>
</template>
