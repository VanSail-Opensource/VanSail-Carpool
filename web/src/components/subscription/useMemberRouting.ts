import { computed, onMounted, onScopeDispose, ref, shallowRef, toRaw, watch } from 'vue'
import { xnodeMemberApi, type RoutingChoice, type RoutingPath, type RoutingView } from '@/api/xnodeMember'
import { memberOperationsApi } from '@/api/memberOperations'
import { ApiError } from '@/api/http'
import { useDurableCommand } from '@/composables/useDurableCommand'
import { localizedError, t } from '@/i18n'
import { createUuid } from '@/utils/browserCompatibility'

export function routingError(caught: unknown, choices: RoutingChoice[] = []) {
  const codes = ['routing_loop', 'routing_overlap_unproven', 'routing_analysis_limit', 'routing_invalid_rule', 'routing_context_unproven']
  if (!(caught instanceof ApiError) || !codes.includes(caught.code)) return localizedError(caught, 'managedRouting.saveFailed')
  const indices = (caught.details?.rules?.split(',') ?? []).map(id => choices.findIndex(c => c.id === id) + 1).filter(index => index > 0)
  const message = t(`managedRouting.errors.${caught.code}`)
  return indices.length ? `${message} ${t('managedRouting.involvedRules', { rules: indices.join(' → ') })}` : message
}
export function useMemberRouting() {
  const current = shallowRef<RoutingView | null>(null)
  const choices = ref<RoutingChoice[]>([])
  const paths = shallowRef<RoutingPath[]>([])
  const trace = shallowRef<RoutingPath | null>(null)
  const loading = shallowRef(true), previewing = shallowRef(false)
  const error = shallowRef<string | null>(null), previewError = shallowRef<string | null>(null)
  const destination = shallowRef(''), start = shallowRef(''), filterNode = shallowRef('')
  let generation = 0, previewGeneration = 0, disposed = false, timer: ReturnType<typeof setTimeout> | undefined
  let reader: AbortController | undefined, previewReader: AbortController | undefined
  const command = useDurableCommand({ errorKey: 'managedRouting.saveFailed', onTerminal: () => load() })
  const busy = computed(() => command.blocksMutations.value || command.submitting.value)
  const nodes = computed(() => current.value?.nodes.map(node => ({ value: node.id, label: node.name })) ?? [])
  const templates = computed(() => current.value?.templates.map(item => ({ value: item.id, label: item.name })) ?? [])
  const locked = computed(() => busy.value || !!current.value && !['active', 'disabled'].includes(current.value.plan.phase))
  const canSave = computed(() => !!current.value && !loading.value && !locked.value && !previewing.value && !previewError.value && !error.value && choices.value.every(c => c.inboundNodeId && c.outboundNodeId && c.templateId && c.inboundNodeId !== c.outboundNodeId))
  async function load() {
    const ticket = ++generation; reader?.abort(); reader = new AbortController()
    loading.value = true; error.value = null
    try {
      const value = await xnodeMemberApi.routing(reader.signal)
      if (disposed || ticket !== generation) return
      current.value = value; choices.value = globalThis.structuredClone(value.plan.choices); paths.value = value.paths
      if (!start.value || !value.nodes.some(n => n.id === start.value)) start.value = value.nodes[0]?.id ?? ''
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
      const text = destination.value.trim()
      const value = await xnodeMemberApi.previewRouting({ choices: choices.value, start: text ? start.value : '', destination: { domain: text, ip: text, port: 443, network: 'tcp' } }, previewReader.signal)
      if (!disposed && ticket === previewGeneration) { paths.value = value.paths; trace.value = value.trace }
    } catch (caught) { if (!disposed && ticket === previewGeneration) { paths.value = []; previewError.value = routingError(caught, choices.value) } }
    finally { if (!disposed && ticket === previewGeneration) previewing.value = false }
  }
  function schedulePreview() {
    previewGeneration++; previewReader?.abort(); if (timer) clearTimeout(timer)
    previewing.value = true; trace.value = null; timer = setTimeout(() => void preview(), 300)
  }
  function add() {
    const a = nodes.value[0]?.value ?? '', b = nodes.value.find(n => n.value !== a)?.value ?? ''
    choices.value = [...choices.value, { id: createUuid(), inboundNodeId: a, outboundNodeId: b, templateId: templates.value[0]?.value ?? '' }]
  }
  function move(index: number, delta: number) { const next = [...choices.value], target = index + delta; if (target < 0 || target >= next.length) return; [next[index], next[target]] = [next[target]!, next[index]!]; choices.value = next }
  async function save() {
    if (!canSave.value || !current.value) return
    const body = { revision: current.value.plan.revision, choices: globalThis.structuredClone(toRaw(choices.value)) }
    await command.execute('routing', JSON.stringify(body), key => xnodeMemberApi.saveRouting(body, key))
  }
  watch([choices, destination, start], schedulePreview, { deep: true })
  onMounted(load)
  onScopeDispose(() => { disposed = true; generation++; previewGeneration++; reader?.abort(); previewReader?.abort(); if (timer) clearTimeout(timer); paths.value = []; trace.value = null })
  return { current, choices, paths, trace, nodes, templates, loading, previewing, error, previewError, destination, start, filterNode, command, locked, canSave, load, add, move, save }
}
