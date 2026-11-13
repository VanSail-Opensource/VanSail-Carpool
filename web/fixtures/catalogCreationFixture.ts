import { ApiError } from '@/api/http'
import { xnodeApi, type XNodeConfiguration, type XNodeSnippet } from '@/api/xnode'

type Trace = { calls: string[]; writes: unknown[] }
type Revisioned = XNodeConfiguration | XNodeSnippet

export function catalogCreationFixture(params: URLSearchParams, trace: Trace, configurations: XNodeConfiguration[]): void {
  const mode = params.get('creation-retry')
  if (mode !== 'configuration' && mode !== 'snippet') return
  const snippets: XNodeSnippet[] = []
  const records: Revisioned[] = mode === 'configuration' ? configurations : snippets
  const attempts: Revisioned[] = []
  let loseResponse = true, unavailableRead = false
  const read = async () => {
    trace.calls.push('creation-read:' + mode)
    if (unavailableRead) { unavailableRead = false; throw new ApiError(503, { code: 'XNODE_UNAVAILABLE', message: '' }) }
    return { items: structuredClone(records) }
  }
  const save = async (input: Partial<Revisioned>) => {
    const item = structuredClone(input) as Revisioned
    attempts.push(item)
    if (!item.id || item.revision !== 0 || records.some(saved => saved.id === item.id || saved.name === item.name)) throw new ApiError(409, { code: 'XNODE_CONFLICT', message: '' })
    const saved = { ...item, revision: 1 }
    records.push(saved)
    trace.writes.push({ creation: mode, item: saved })
    if (loseResponse) { loseResponse = false; unavailableRead = true; throw new TypeError('Constructed response loss after commit') }
    return structuredClone(saved)
  }
  if (mode === 'configuration') {
    xnodeApi.configurations = async () => await read() as { items: XNodeConfiguration[] }
    xnodeApi.saveConfiguration = async input => await save(input) as XNodeConfiguration
  } else {
    xnodeApi.snippets = async () => await read() as { items: XNodeSnippet[] }
    xnodeApi.saveSnippet = async input => await save(input) as XNodeSnippet
  }
  Object.assign(window, { __catalogCreation: { attempts, records } })
}
