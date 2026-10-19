<script setup lang="ts">
import { computed } from 'vue'
import type { RoutingChoice, RoutingPath, RoutingView } from '@/api/xnodeMember'
import { useI18n } from '@/i18n'
import { endpointAddress, selectorLabel } from './routingPresentation'

const props = defineProps<{ paths: RoutingPath[]; choices: RoutingChoice[]; nodes: RoutingView['nodes']; endpoints: NonNullable<RoutingView['endpoints']>; templates: RoutingView['templates'] }>()
const { t } = useI18n()
const rows = computed(() => {
  const choices = new Map(props.choices.map(choice => [choice.id, choice]))
  const templates = new Map(props.templates.map(template => [template.id, template]))
  const nodes = new Map(props.nodes.map(node => [node.id, node.name]))
  const endpoints = new Map(props.endpoints.map(endpoint => [endpoint.id, endpoint]))
  const label = (ids: string[]) => selectorLabel(ids.flatMap(id => { const template = templates.get(choices.get(id)?.templateId ?? ''); return template ? [template.match] : [] }), t('managedRouting.allTraffic'))
  return props.paths.map(path => ({
    destination: label(path.rules), excluded: path.excludedRules.length ? label(path.excludedRules) : '',
    hops: path.nodes.map((id, hop) => {
      const choice = choices.get(path.rules[Math.max(0, hop - 1)] ?? '')
      const endpoint = endpoints.get((hop === 0 ? choice?.inboundEndpointId : choice?.outboundEndpointId) ?? '')
      return { name: nodes.get(id) ?? id, host: endpointAddress(endpoint), title: endpoint?.name }
    }),
  }))
})
</script>

<template>
  <ol v-if="rows.length" class="routing-paths grid gap-3 text-sm">
    <li v-for="(path, index) in rows" :key="index" class="grid gap-1">
      <div class="flex flex-wrap items-start gap-x-2 gap-y-1">
        <span>{{ t('managedRouting.you') }}</span><span aria-hidden="true" class="text-dimmed">→</span>
        <template v-for="(hop, number) in path.hops" :key="number">
          <span class="grid min-w-0"><span>{{ hop.name }}</span><small v-if="hop.host" :title="hop.title" class="break-all text-xs text-muted">{{ hop.host }}</small></span>
          <span aria-hidden="true" class="text-dimmed">→</span>
        </template>
        <span class="break-words font-medium">{{ path.destination }}</span>
      </div>
      <p v-if="path.excluded" class="text-xs text-muted">{{ t('managedRouting.except', { conditions: path.excluded }) }}</p>
    </li>
  </ol>
</template>
