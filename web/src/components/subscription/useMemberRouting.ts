import { computed, onMounted, onScopeDispose, ref, shallowRef, watch } from 'vue'
import { xnodeMemberApi, type RoutingChoice, type RoutingPath, type RoutingView } from '@/api/xnodeMember'
import { memberOperationsApi } from '@/api/memberOperations'
import { ApiError } from '@/api/http'
import { useDurableCommand } from '@/composables/useDurableCommand'
import { localizedError, t } from '@/i18n'
import { createUuid } from '@/utils/browserCompatibility'

export function routingError(caught: unknown, choices: RoutingChoice[] = []) {
  const codes = ['routing_loop', 'routing_overlap_unproven', 'routing_analysis_limit', 'routing_invalid_rule', 'routing_context_unproven', 'routing_endpoint_unavailable']
  if (!(caught instanceof ApiError) || !codes.includes(caught.code)) return localizedError(caught, 'managedRouting.saveFailed')
  const indices = (caught.details?.rules?.split(',') ?? []).map(id => choices.findIndex(c => c.id === id) + 1).filter(index => index > 0)
  const message = t(`managedRouting.errors.${caught.code}`)
  return indices.length ? `${message} ${t('managedRouting.involvedRules', { rules: indices.join(' → ') })}` : message
}
export function useMemberRouting(options?: { initial: () => RoutingView | undefined; onDraft: (choices: RoutingChoice[]) => void; onRefresh: () => void }) {
  const current = shallowRef<RoutingView | null>(null)
  let savedChoices = ''
  const choices = ref<RoutingChoice[]>([])
  const paths = shallowRef<RoutingPath[]>([])
  const trace = shallowRef<RoutingPath | null>(null)
  const loading = shallowRef(true), previewing = shallowRef(false)
  const error = shallowRef<string | null>(null), previewError = shallowRef<string | null>(null)
  const destination = shallowRef(''), start = shallowRef(''), filterNode = shallowRef('')
  const startEndpoint = shallowRef('')
  const entryHosts = new Map<string, string>()
  let generation = 0, previewGeneration = 0, disposed = false, timer: ReturnType<typeof setTimeout> | undefined
  let reader: AbortController | undefined, previewReader: AbortController | undefined
  const command = useDurableCommand({ errorKey: 'managedRouting.saveFailed', onTerminal: () => { if (options) options.onRefresh(); return load() } })
  const busy = computed(() => command.blocksMutations.value || command.submitting.value)
  const nodes = computed(() => current.value?.nodes.map(node => ({ value: node.id, label: node.name })) ?? [])
  const endpoints = computed(() => current.value?.endpoints ?? [])
  const endpointItems = computed(() => endpoints.value.map(endpoint => ({ value: endpoint.id, nodeId: endpoint.nodeId, label: endpoint.name })))
  const traceNodes = computed(() => nodes.value.filter(node => choices.value.some(choice => choice.inboundNodeId === node.value)))
  const traceEndpoints = computed(() => endpointItems.value.filter(endpoint => choices.value.some(choice => choice.inboundNodeId === start.value && choice.inboundEndpointId === endpoint.value)))
  const templates = computed(() => current.value?.templates.map(item => ({ value: item.id, label: item.name })) ?? [])
  const locked = computed(() => busy.value || !!current.value && !['active', 'disabled'].includes(current.value.plan.phase))
  const routeStatus = computed(() => previewError.value ? 'error' : previewing.value ? 'checking' : locked.value || JSON.stringify(choices.value) !== savedChoices ? 'draft' : current.value?.plan.phase === 'disabled' ? 'disabled' : 'active')
  const canSave = computed(() => !!current.value && !loading.value && !locked.value && !previewing.value && !previewError.value && !error.value && choices.value.every(c => c.inboundEndpointId && c.outboundEndpointId && c.inboundNodeId && c.outboundNodeId && c.templateId && c.inboundNodeId !== c.outboundNodeId))
  const canSpeedtest = computed(() => routeStatus.value === 'active' && !current.value?.plan.reason && !!trace.value?.rules.length && !!startEndpoint.value && !loading.value && !error.value)
  async function load() {
    const ticket = ++generation; reader?.abort(); reader = new AbortController()
    loading.value = true; error.value = null
    try {
      const value = options ? options.initial() : await xnodeMemberApi.routing(reader.signal)
      if (!value) return
      if (disposed || ticket !== generation) return
      const keepDraft = !!options && !!current.value && JSON.stringify(choices.value) !== savedChoices
      current.value = value
      if (['active', 'disabled'].includes(value.plan.phase) && value.plan.operationId === command.receipt.value?.id && !command.submitting.value) command.reset()
      const available = [...(value.endpoints ?? [])].sort((a, b) => a.id < b.id ? -1 : a.id > b.id ? 1 : 0)
      const incoming = globalThis.structuredClone(value.plan.choices).map(choice => ({ ...choice, inboundEndpointId: choice.inboundEndpointId || available.find(endpoint => endpoint.nodeId === choice.inboundNodeId)?.id, outboundEndpointId: choice.outboundEndpointId || available.find(endpoint => endpoint.nodeId === choice.outboundNodeId)?.id }))
      if (!keepDraft) choices.value = incoming
      savedChoices = JSON.stringify(incoming)
      for (const choice of choices.value) if (choice.inboundEndpointId && !entryHosts.has(choice.inboundNodeId)) entryHosts.set(choice.inboundNodeId, choice.inboundEndpointId)
      paths.value = value.paths
      if (!traceNodes.value.some(node => node.value === start.value)) start.value = traceNodes.value[0]?.value ?? ''
      if (value.plan.operationId && !['active', 'disabled'].includes(value.plan.phase)) {
        const receipt = await memberOperationsApi.getOperation(value.plan.operationId)
        if (!disposed && ticket === generation) command.resume(receipt, 'routing')
      }
    } catch (caught) { if (!disposed && ticket === generation) { current.value = null; error.value = localizedError(caught, 'xnode.loadFailed') } }
    finally { if (!disposed && ticket === generation) loading.value = false }
  }
  async function preview() {
    const ticket = ++previewGeneration; previewReader?.abort(); previewReader = new AbortController()
    if (!current.value) { previewing.value = false; return }
    previewing.value = true; previewError.value = null; trace.value = null
    try {
      const text = traceNodes.value.length ? destination.value.trim() : ''
      const value = await xnodeMemberApi.previewRouting({ choices: choices.value, start: text ? start.value : '', startEndpointId: text ? startEndpoint.value : '', destination: { domain: text, ip: text, port: 443, network: 'tcp' } }, previewReader.signal)
      if (!disposed && ticket === previewGeneration) { paths.value = value.paths; trace.value = value.trace }
    } catch (caught) { if (!disposed && ticket === previewGeneration) { paths.value = []; previewError.value = routingError(caught, choices.value) } }
    finally { if (!disposed && ticket === previewGeneration) previewing.value = false }
  }
  function schedulePreview() {
    previewGeneration++; previewReader?.abort(); if (timer) clearTimeout(timer)
    previewing.value = true; trace.value = null; timer = setTimeout(() => void preview(), 300)
  }
  function add() {
    const a = filterNode.value || start.value || nodes.value[0]?.value || ''
    const existing = choices.value.find(choice => choice.inboundNodeId === a)
    const b = existing?.outboundNodeId ?? nodes.value.find(node => node.value !== a)?.value ?? ''
    const remembered = entryHosts.get(a)
    const source = endpoints.value.find(endpoint => endpoint.id === remembered && endpoint.nodeId === a) ?? endpoints.value.find(endpoint => endpoint.nodeId === a)
    const target = endpoints.value.find(endpoint => endpoint.id === existing?.outboundEndpointId && endpoint.nodeId === b) ?? endpoints.value.find(endpoint => endpoint.nodeId === b)
    choices.value = [...choices.value, { id: createUuid(), inboundNodeId: a, outboundNodeId: b, inboundEndpointId: source?.id, outboundEndpointId: target?.id, templateId: templates.value[0]?.value ?? '' }]
  }
  function setEndpoint(choice: RoutingChoice, side: 'inbound' | 'outbound', id: string) {
    const endpoint = endpoints.value.find(endpoint => endpoint.id === id)
    if (!endpoint) return
    if (side === 'inbound') { choice.inboundNodeId = endpoint.nodeId; choice.inboundEndpointId = id; entryHosts.set(endpoint.nodeId, id) }
    else { choice.outboundNodeId = endpoint.nodeId; choice.outboundEndpointId = id }
  }
  function selectNode(id: string) { filterNode.value = filterNode.value === id ? '' : id; if (filterNode.value && traceNodes.value.some(node => node.value === id)) start.value = id }
  function move(index: number, delta: number) { const next = [...choices.value], target = index + delta; if (target < 0 || target >= next.length) return; [next[index], next[target]] = [next[target]!, next[index]!]; choices.value = next }
  async function save() {
    if (!canSave.value || !current.value) return
    const body = { revision: current.value.plan.revision, choices: choices.value.map(choice => ({ ...choice })) }
    await command.execute('routing', JSON.stringify(body), key => xnodeMemberApi.saveRouting(body, key))
  }
  watch(traceNodes, value => { if (!value.some(node => node.value === start.value)) start.value = value[0]?.value ?? '' })
  watch([start, traceEndpoints], () => { if (!traceEndpoints.value.some(endpoint => endpoint.value === startEndpoint.value)) startEndpoint.value = traceEndpoints.value.find(endpoint => endpoint.value === entryHosts.get(start.value))?.value ?? traceEndpoints.value[0]?.value ?? '' })
  watch(startEndpoint, id => { if (endpoints.value.some(endpoint => endpoint.id === id && endpoint.nodeId === start.value)) entryHosts.set(start.value, id) })
  watch([choices, destination, start, startEndpoint], schedulePreview, { deep: true })
  if (options) {
    watch(options.initial, () => void load())
    watch(choices, value => options.onDraft(globalThis.structuredClone(value.map(choice => ({ ...choice })))), { deep: true })
  }
  onMounted(load)
  onScopeDispose(() => { disposed = true; generation++; previewGeneration++; reader?.abort(); previewReader?.abort(); if (timer) clearTimeout(timer); paths.value = []; trace.value = null })
  return { current, choices, routeStatus, paths, trace, nodes, traceNodes, traceEndpoints, endpoints, endpointItems, templates, loading, previewing, error, previewError, destination, start, startEndpoint, filterNode, command, locked, canSave, canSpeedtest, load, add, move, save, setEndpoint, selectNode }
}
