import type { components } from './generated'
import { request } from './http'

export type EmbyConnection = components['schemas']['EmbyConnection']
export type EmbyConnectionWrite = components['schemas']['EmbyConnectionWrite']

export const embyConnectionApi = {
  get: () => request<EmbyConnection>('/api/v1/admin/emby-connection'),
  save: (body: EmbyConnectionWrite) => request<EmbyConnection>('/api/v1/admin/emby-connection', { method: 'PUT', body }),
}
