import type { components } from './generated'
import type { DeepReadonly } from './types'
import { request } from './http'

export type PAYGView = DeepReadonly<components['schemas']['PAYGView']>
export type PAYGSquad = PAYGView['squads'][number]
export const paygApi = {
  get: (signal?: AbortSignal) => request<PAYGView>('/api/v1/payg', { signal, cache: 'no-store' }),
}
