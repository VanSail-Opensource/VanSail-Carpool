<script setup lang="ts">
import { computed, reactive, watch } from 'vue'
import { z } from 'zod'
import type { XNodeSettings } from '@/api/xnode'
import { useI18n } from '@/i18n'

type Policy = NonNullable<XNodeSettings['firewall']>
type ListKey = 'tcp_ports' | 'udp_ports' | 'ssh_ports' | 'allow_prefixes'
const model = defineModel<Policy>({ required: true })
const emit = defineEmits<{ valid: [value: boolean] }>()
const { t } = useI18n()
const keys: ListKey[] = ['tcp_ports', 'udp_ports', 'ssh_ports', 'allow_prefixes']
const text = reactive<Record<ListKey, string>>({ tcp_ports: '', udp_ports: '', ssh_ports: '', allow_prefixes: '' })
const invalid = reactive<Record<ListKey, boolean>>({ tcp_ports: false, udp_ports: false, ssh_ports: false, allow_prefixes: false })
const prefix = z.union([z.cidrv4(), z.cidrv6()])
const ip = z.union([z.ipv4(), z.ipv6()])
for (const key of keys) watch(() => model.value[key]?.join(', ') ?? '', value => { text[key] = value; invalid[key] = false }, { immediate: true })
function update(key: ListKey, value: string) {
  text[key] = value
  const tokens = value.split(/[\s,]+/).filter(Boolean)
  if (key === 'allow_prefixes') {
    invalid[key] = tokens.some(item => !prefix.safeParse(item).success)
    if (!invalid[key]) model.value = { ...model.value, allow_prefixes: [...new Set(tokens)] }
  } else {
    invalid[key] = tokens.some(item => !/^\d+$/.test(item) || Number(item) < 1 || Number(item) > 65535)
    if (!invalid[key]) model.value = { ...model.value, [key]: [...new Set(tokens.map(Number))] }
  }
}
const forwards = computed({ get: () => model.value.forward ?? [], set: value => { model.value = { ...model.value, forward: value } } })
const valid = computed(() => !Object.values(invalid).some(Boolean) && forwards.value.every(row => ip.safeParse(row.target_ip).success && (!row.listen_ip || ip.safeParse(row.listen_ip).success) && Number.isInteger(row.port) && row.port >= 1 && row.port <= 65535 && Number.isInteger(row.target_port) && row.target_port >= 1 && row.target_port <= 65535))
watch(valid, value => emit('valid', value), { immediate: true })
function addForward() { forwards.value = [...forwards.value, { port: 443, target_port: 443, target_ip: '', listen_ip: '', network: 'tcp' }] }
</script>

<template>
  <div class="grid gap-4">
    <div class="grid gap-4 sm:grid-cols-2">
      <UFormField v-for="key in keys" :key="key" :label="t(`nodeForms.${key}`)" :error="invalid[key] ? t('nodeForms.invalidList') : undefined">
        <UTextarea :model-value="text[key]" :rows="2" class="w-full" @update:model-value="update(key, String($event))" />
      </UFormField>
    </div>
    <div class="flex items-center justify-between"><span class="text-sm font-medium">{{ t('nodeForms.forwards') }}</span><UButton color="neutral" variant="ghost" icon="i-ph-plus" :aria-label="t('nodeForms.addForward')" @click="addForward" /></div>
    <div v-for="(row, index) in forwards" :key="index" class="grid gap-3 border-b border-muted pb-4 sm:grid-cols-2">
      <UFormField :label="t('nodeForms.listenAddress')"><UInput v-model="row.listen_ip" class="w-full" /></UFormField>
      <UFormField :label="t('xnode.port')"><UInput v-model.number="row.port" type="number" min="1" max="65535" required class="w-full" /></UFormField>
      <UFormField :label="t('nodeForms.targetAddress')"><UInput v-model="row.target_ip" required class="w-full" /></UFormField>
      <UFormField :label="t('nodeForms.targetPort')"><UInput v-model.number="row.target_port" type="number" min="1" max="65535" required class="w-full" /></UFormField>
      <UFormField :label="t('nodeForms.network')"><USelect v-model="row.network" :items="['tcp', 'udp']" class="w-full" /></UFormField>
      <UButton class="justify-self-end self-end" color="error" variant="ghost" icon="i-ph-trash" :aria-label="t('common.delete')" @click="forwards = forwards.filter((_, position) => position !== index)" />
    </div>
  </div>
</template>
