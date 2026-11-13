import { beforeEach, describe, expect, it, vi } from 'vitest'
import { ApiError } from '@/api/http'
import type { XNodeConfiguration, XNodeSnippet } from '@/api/xnode'
import { saveConfigurationWithRecovery, saveSnippetWithRecovery } from './catalogCreation'

const api = vi.hoisted(() => ({ saveConfiguration: vi.fn(), configurations: vi.fn(), saveSnippet: vi.fn(), snippets: vi.fn() }))
vi.mock('@/api/xnode', () => ({ xnodeApi: api }))
const conflict = new ApiError(409, { code: 'XNODE_CONFLICT', message: 'Conflict' })
const input: XNodeConfiguration = { id: 'new-config', name: 'Created', revision: 0, snippetIds: ['first', 'second'], document: { inbounds: [], routing: { rules: [{ type: 'field', outboundTag: 'direct' }] } } }
const snippet: XNodeSnippet = { id: 'new-snippet', name: 'Created', revision: 0, kind: 'outbound', document: [{ tag: 'direct', protocol: 'freedom' }] }
beforeEach(() => { vi.resetAllMocks() })

describe('native catalog creation recovery', () => {
  it('confirms matching creation content from the same ID without changing its expected revision', async () => {
    api.saveConfiguration.mockRejectedValue(new TypeError('Lost response'))
    const saved = { ...input, revision: 1, document: { routing: { rules: [{ outboundTag: 'direct', type: 'field' }] }, inbounds: [] } }
    api.configurations.mockResolvedValue({ items: [{ ...saved, id: 'different-id' }, saved] })
    expect(await saveConfigurationWithRecovery(input)).toBe(saved)
    expect(api.saveConfiguration).toHaveBeenCalledExactlyOnceWith(input)
    expect(api.configurations).toHaveBeenCalledOnce()
    expect(input.revision).toBe(0)
  })

  it.each([
    { name: 'Changed' },
    { document: { inbounds: [], routing: { rules: [] } } },
    { snippetIds: ['second', 'first'] },
    { id: 'different-id' },
    { revision: 0 },
  ])('does not accept or overwrite differing configuration state: %j', async change => {
    api.saveConfiguration.mockRejectedValue(conflict)
    api.configurations.mockResolvedValue({ items: [{ ...input, revision: 1, ...change }] })
    await expect(saveConfigurationWithRecovery(input)).rejects.toBe(conflict)
    expect(api.saveConfiguration).toHaveBeenCalledExactlyOnceWith(input)
    expect(input.revision).toBe(0)
  })

  it('compares snippet kind and ordered document entries when reconciling a revision conflict', async () => {
    api.saveSnippet.mockRejectedValue(conflict)
    const saved = { ...snippet, revision: 1 }
    api.snippets.mockResolvedValue({ items: [saved] })
    expect(await saveSnippetWithRecovery(snippet)).toBe(saved)
    api.snippets.mockResolvedValue({ items: [{ ...saved, kind: 'routing' }] })
    await expect(saveSnippetWithRecovery(snippet)).rejects.toBe(conflict)
    const ordered = { ...snippet, document: [{ tag: 'first' }, { tag: 'second' }] }
    api.snippets.mockResolvedValue({ items: [{ ...ordered, revision: 1, document: [...ordered.document].reverse() }] })
    await expect(saveSnippetWithRecovery(ordered)).rejects.toBe(conflict)
  })

  it('preserves the original uncertainty when the recovery read fails or returns no matching resource', async () => {
    const unavailable = new ApiError(503, { code: 'XNODE_UNAVAILABLE', message: 'Unavailable' })
    api.saveConfiguration.mockRejectedValue(unavailable)
    api.configurations.mockRejectedValueOnce(new Error('Read failed')).mockResolvedValue({ items: [] })
    await expect(saveConfigurationWithRecovery(input)).rejects.toBe(unavailable)
    await expect(saveConfigurationWithRecovery(input)).rejects.toBe(unavailable)
  })

  it.each([401, 403, 422, 429])('does not reinterpret definitive HTTP %i refusal as creation success', async status => {
    const refused = new ApiError(status, { code: 'REFUSED', message: 'Refused' })
    api.saveConfiguration.mockRejectedValue(refused)
    await expect(saveConfigurationWithRecovery(input)).rejects.toBe(refused)
    expect(api.configurations).not.toHaveBeenCalled()
  })

  it('does not bypass revision conflicts for an existing edit', async () => {
    api.saveConfiguration.mockRejectedValue(conflict)
    await expect(saveConfigurationWithRecovery({ ...input, revision: 3 })).rejects.toBe(conflict)
    expect(api.configurations).not.toHaveBeenCalled()
  })

  it('stops recovery after disposal and does not perform a read after a confirmed save', async () => {
    const controller = new AbortController()
    api.saveConfiguration.mockImplementation(async () => { controller.abort(); throw conflict })
    await expect(saveConfigurationWithRecovery(input, controller.signal)).rejects.toBe(conflict)
    expect(api.configurations).not.toHaveBeenCalled()
    const saved = { ...input, revision: 1 }
    api.saveConfiguration.mockResolvedValue(saved)
    expect(await saveConfigurationWithRecovery(input)).toBe(saved)
    expect(api.configurations).not.toHaveBeenCalled()
  })
})
