import type { components } from './generated'
import type { DeepReadonly } from './types'
import { request } from './http'

export type PAYGNodeHistory = DeepReadonly<components['schemas']['XNodePAYGNodeHistory']>
export type PAYGView = DeepReadonly<components['schemas']['PAYGView']>
export type PAYGSquad = PAYGView['squads'][number]
export const paygApi = {
  history: (signal?: AbortSignal) => request<PAYGNodeHistory>('/api/v1/payg/history', { signal, cache: 'no-store' }),
  get: (signal?: AbortSignal) => request<PAYGView>('/api/v1/payg', { signal, cache: 'no-cache' }),
}
