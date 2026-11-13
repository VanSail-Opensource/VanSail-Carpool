import { ApiError } from '@/api/http'
import { xnodeApi, type XNodeConfiguration, type XNodeSnippet } from '@/api/xnode'

// JSON object order is immaterial; array order remains part of the definition.
function sameJSON(left: unknown, right: unknown): boolean {
  const pending: Array<[unknown, unknown]> = [[left, right]]
  while (pending.length) {
    const [one, two] = pending.pop()!
    if (one === two) continue
    if (one === null || two === null || typeof one !== 'object' || typeof two !== 'object') return false
    if (Array.isArray(one) !== Array.isArray(two)) return false
    if (Array.isArray(one) && one.length !== (two as unknown[]).length) return false
    const keys = Object.keys(one)
    if (keys.length !== Object.keys(two).length) return false
    for (const key of keys) {
      if (!Object.hasOwn(two, key)) return false
      pending.push([(one as Record<string, unknown>)[key], (two as Record<string, unknown>)[key]])
    }
  }
  return true
}

type Revisioned = { id: string; name: string; revision: number }
async function saveCreation<T extends Revisioned>(input: T, save: () => Promise<T>, read: () => Promise<{ items: T[] }>, matches: (saved: T) => boolean, signal?: AbortSignal): Promise<T> {
  try { return await save() }
  catch (caught) {
    // Existing edits must retain optimistic concurrency. Only new resources
    // with an explicit stable identity can reconcile an uncertain creation.
    if (input.revision !== 0 || !input.id || signal?.aborted) throw caught
    if (caught instanceof ApiError && ![408, 409].includes(caught.status) && caught.status < 500) throw caught
    let saved: T | undefined
    try { saved = (await read()).items.find(item => item.id === input.id) }
    catch { throw caught }
    if (signal?.aborted || !saved || saved.revision < 1 || saved.name !== input.name || !matches(saved)) throw caught
    return saved
  }
}

export function saveConfigurationWithRecovery(input: XNodeConfiguration, signal?: AbortSignal): Promise<XNodeConfiguration> {
  return saveCreation(input, () => xnodeApi.saveConfiguration(input), () => xnodeApi.configurations(signal),
    saved => sameJSON(saved.document, input.document) && sameJSON(saved.snippetIds, input.snippetIds), signal)
}

export function saveSnippetWithRecovery(input: XNodeSnippet, signal?: AbortSignal): Promise<XNodeSnippet> {
  return saveCreation(input, () => xnodeApi.saveSnippet(input), () => xnodeApi.snippets(signal),
    saved => saved.kind === input.kind && sameJSON(saved.document, input.document), signal)
}
