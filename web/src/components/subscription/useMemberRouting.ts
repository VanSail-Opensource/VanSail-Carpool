import { computed, onMounted, onScopeDispose, shallowRef, watch } from 'vue'
import { xnodeMemberApi, type XNodeCustomization } from '@/api/xnodeMember'
import { localizedError, t } from '@/i18n'

export function useMemberRouting() {
  type Template = Awaited<ReturnType<typeof xnodeMemberApi.templates>>['items'][number]
  const templates = shallowRef<Template[]>([])
  const node = shallowRef('')
  const current = shallowRef<XNodeCustomization | null>(null)
  const loading = shallowRef(true)
  const busy = shallowRef(false)
  const error = shallowRef<string | null>(null)
  const saved = shallowRef(false)
  const outbounds = shallowRef<unknown>([])
  const rules = shallowRef<unknown>([])
  let generation = 0
  let disposed = false
  const nodes = computed(() => [...new Map(templates.value.map(item => [item.nodeId, { value: item.nodeId, label: item.name }])).values()])
  async function loadNode() {
    const ticket = ++generation
    current.value = null; saved.value = false; error.value = null
    if (!node.value) return
    loading.value = true
    try {
      const value = await xnodeMemberApi.routing(node.value)
      if (ticket !== generation || disposed) return
      current.value = value; outbounds.value = value.outbounds; rules.value = value.routing
    } catch (caught) { if (ticket === generation && !disposed) error.value = localizedError(caught, 'xnode.loadFailed') }
    finally { if (ticket === generation && !disposed) loading.value = false }
  }
  async function load() {
    loading.value = true; error.value = null
    try {
      const value = await xnodeMemberApi.templates()
      if (disposed) return
      templates.value = value.items
      if (!nodes.value.some(item => item.value === node.value)) node.value = nodes.value[0]?.value ?? ''
      else await loadNode()
    } catch (caught) { if (!disposed) error.value = localizedError(caught, 'xnode.loadFailed') }
    finally { if (!disposed) loading.value = false }
  }
  async function save(kind: 'outbounds' | 'rules') {
    if (busy.value || !current.value) return
    const objects = kind === 'outbounds' ? outbounds.value : rules.value
    if (!Array.isArray(objects) || objects.some(item => !item || typeof item !== 'object' || Array.isArray(item))) { error.value = t('memberNodes.arrayRequired'); return }
    busy.value = true; error.value = null; saved.value = false
    const ticket = generation
    try {
      const value = await xnodeMemberApi.saveRouting(node.value, kind, { revision: current.value.revision, objects })
      if (disposed || ticket !== generation) return
      current.value = value; saved.value = true
    } catch (caught) { if (!disposed && ticket === generation) error.value = localizedError(caught, 'memberNodes.saveFailed') }
    finally { if (!disposed) busy.value = false }
  }
  function insert(index: number) {
    const template = templates.value[index]
    if (!template || !Array.isArray(outbounds.value)) return
    const outbound = structuredClone(template.outbound)
    const tags = new Set(outbounds.value.map(item => item?.tag))
    const base = String(outbound.tag ?? 'peer')
    let tag = base
    for (let suffix = 2; tags.has(tag); suffix++) tag = `${base}-${suffix}`
    outbounds.value = [...outbounds.value, { ...outbound, tag }]
    saved.value = false
  }
  watch(node, loadNode)
  watch([outbounds, rules], () => { saved.value = false })
  onMounted(load)
  onScopeDispose(() => { disposed = true; generation++ })
  return { templates, node, nodes, current, loading, busy, error, saved, outbounds, rules, load, save, insert }
}
