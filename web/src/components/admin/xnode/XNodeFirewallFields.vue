<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { z } from 'zod'
import { useI18n } from '@/i18n'
import { firewallPortKeys, firewallPortsText, firewallRangeKeys, parseFirewallPorts, type FirewallPolicy, type FirewallPortKey } from './firewallPorts'
import { firewallForwardDraft, parseFirewallForwards, type FirewallForwardDraft, type FirewallForwardErrors } from './firewallForwarding'

const model = defineModel<FirewallPolicy>({ required: true })
const emit = defineEmits<{ valid: [value: boolean] }>()
const { t } = useI18n()
type ListKey = FirewallPortKey | 'allow_prefixes'
const keys: ListKey[] = [...firewallPortKeys, 'allow_prefixes']
const text = reactive<Record<ListKey, string>>({ tcp_ports: '', udp_ports: '', ssh_ports: '', allow_prefixes: '' })
const invalid = reactive<Record<ListKey, boolean>>({ tcp_ports: false, udp_ports: false, ssh_ports: false, allow_prefixes: false })
const publishedLists = new Map<ListKey, string>()
const prefix = z.union([z.cidrv4(), z.cidrv6()])
for (const key of keys) watch(() => JSON.stringify(key === 'allow_prefixes' ? model.value[key] : [model.value[key], model.value[firewallRangeKeys[key]]]), signature => {
  if (signature === publishedLists.get(key)) return
  text[key] = key === 'allow_prefixes' ? model.value[key]?.join(', ') ?? '' : firewallPortsText(model.value, key)
  invalid[key] = key === 'allow_prefixes' ? text[key].split(/[\s,]+/).filter(Boolean).some(item => !prefix.safeParse(item).success) : !parseFirewallPorts(text[key])
}, { immediate: true })

function update(key: ListKey, value: string): void {
  text[key] = value
  if (key === 'allow_prefixes') {
    const tokens = value.split(/[\s,]+/).filter(Boolean)
    invalid[key] = tokens.some(item => !prefix.safeParse(item).success)
    if (invalid[key]) return
    const prefixes = [...new Set(tokens)]
    publishedLists.set(key, JSON.stringify(prefixes))
    model.value = { ...model.value, allow_prefixes: prefixes }
  } else {
    const result = parseFirewallPorts(value)
    invalid[key] = !result
    if (!result) return
    publishedLists.set(key, JSON.stringify([result.ports, result.ranges]))
    model.value = { ...model.value, [key]: result.ports, [firewallRangeKeys[key]]: result.ranges }
  }
}

const forwards = ref<FirewallForwardDraft[]>([])
let nextID = 0
let publishedForwards = ''
watch(() => JSON.stringify(model.value.forward ?? []), signature => {
  if (signature !== publishedForwards) forwards.value = (model.value.forward ?? []).map(row => firewallForwardDraft(row, nextID++))
}, { immediate: true })
const parsedForwards = computed(() => parseFirewallForwards(forwards.value))
watch(parsedForwards, result => {
  if (!result.valid) return
  const signature = JSON.stringify(result.values)
  if (signature === JSON.stringify(model.value.forward ?? [])) return
  publishedForwards = signature
  model.value = { ...model.value, forward: result.values }
}, { immediate: true, flush: 'sync' })
const valid = computed(() => !Object.values(invalid).some(Boolean) && parsedForwards.value.valid)
watch(valid, value => emit('valid', value), { immediate: true, flush: 'sync' })
function addForward(): void { forwards.value.push(firewallForwardDraft({ port: 443, target_port: 443, target_ip: '', network: 'tcp' }, nextID++)) }
function forwardError(index: number, field: keyof FirewallForwardErrors): string | undefined {
  const error = parsedForwards.value.errors[index]?.[field] ?? (field === 'ports' ? parsedForwards.value.errors[index]?.overlap : undefined)
  return error ? t(`nodeForms.${error}`) : undefined
}
</script>

<template>
  <div class="grid gap-4" data-firewall-fields>
    <div class="grid gap-4 sm:grid-cols-2">
      <UFormField v-for="key in keys" :key="key" :name="`firewall.${key}`" :label="t(`nodeForms.${key}`)" :description="t(key === 'allow_prefixes' ? 'nodeForms.prefixListHint' : 'nodeForms.portListHint')" :error="invalid[key] ? t(key === 'allow_prefixes' ? 'nodeForms.prefixListInvalid' : 'nodeForms.portListInvalid') : undefined">
        <UTextarea :model-value="text[key]" :rows="2" class="w-full" @update:model-value="update(key, String($event))" />
      </UFormField>
    </div>
    <div class="flex items-center justify-between"><span class="text-sm font-medium">{{ t('nodeForms.forwards') }}</span><UButton type="button" color="neutral" variant="ghost" icon="i-ph-plus" :aria-label="t('nodeForms.addForward')" @click="addForward" /></div>
    <p class="text-xs text-muted">{{ t('nodeForms.forwardRangeHint') }}</p>
    <div v-for="(row, index) in forwards" :key="row.id" class="grid gap-3 border-b border-muted pb-4 sm:grid-cols-2" data-firewall-forward>
      <UFormField :name="`firewall.forward.${row.id}.listen`" :label="t('nodeForms.listenAddress')" :error="forwardError(index, 'listen_ip')"><UInput v-model="row.listen_ip" class="w-full" /></UFormField>
      <UFormField :name="`firewall.forward.${row.id}.ports`" :label="t('nodeForms.sourcePorts')" :error="forwardError(index, 'ports')"><UInput v-model="row.ports" required class="w-full" /></UFormField>
      <UFormField :name="`firewall.forward.${row.id}.target`" :label="t('nodeForms.targetAddress')" :error="forwardError(index, 'target_ip')"><UInput v-model="row.target_ip" required class="w-full" /></UFormField>
      <UFormField :name="`firewall.forward.${row.id}.targetPorts`" :label="t('nodeForms.targetPorts')" :error="forwardError(index, 'target_ports')"><UInput v-model="row.target_ports" required class="w-full" /></UFormField>
      <UFormField :name="`firewall.forward.${row.id}.network`" :label="t('nodeForms.network')" :error="forwardError(index, 'network')"><USelect v-model="row.network" :items="['tcp', 'udp']" class="w-full" /></UFormField>
      <UButton type="button" class="justify-self-end self-end" color="error" variant="ghost" icon="i-ph-trash" :aria-label="t('common.delete')" @click="forwards.splice(index, 1)" />
    </div>
  </div>
</template>
