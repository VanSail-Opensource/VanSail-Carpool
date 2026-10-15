import type { components } from './generated'
import { request } from './http'

type Schema = components['schemas']
export type XNode = Schema['XNodeNode']
export type XNodeSettings = Schema['XNodeSettings']
export type XNodeGlobal = Schema['XNodeGlobal']
export type XNodeConfiguration = Schema['XNodeConfiguration']
export type XNodeSnippet = Schema['XNodeSnippet']
export type XNodeGroup = Schema['XNodeGroup']
export type XNodeEndpoint = Schema['XNodeEndpoint']
export type XNodeMetrics = Schema['XNodeMetricsSummary']
export type XNodeValidation = Schema['XNodeValidation']
export type XNodeEnrollment = Schema['XNodeEnrollment']
export type XNodeMigration = Schema['XNodeMigration']
export type XNodeMigrationPreview = Schema['XNodeMigrationPreview']
export type XNodeMigrationPreviewWrite = Schema['XNodeMigrationPreviewWrite']
export type XNodeCommand = Schema['XNodeCommandWrite']

const root = '/api/v1/admin/xnode'
export const xnodeApi = {
  nodes: () => request<{ items: XNode[] }>(`${root}/nodes`, { cache: 'no-store' }),
  node: (id: string) => request<Schema['XNodeNodeDetail']>(`${root}/nodes/${encodeURIComponent(id)}`, { cache: 'no-store' }),
  saveNode: (node: Partial<XNode>, settings: XNodeSettings) => request<XNode>(`${root}/nodes${node.id ? `/${encodeURIComponent(node.id)}` : ''}`, { method: node.id ? 'PUT' : 'POST', body: { node, settings } }),
  enroll: (id: string) => request<XNodeEnrollment>(`${root}/nodes/${encodeURIComponent(id)}/enrollment`, { method: 'POST', cache: 'no-store' }),
  global: () => request<XNodeGlobal>(`${root}/global`, { cache: 'no-store' }),
  saveGlobal: (body: XNodeGlobal) => request<XNodeGlobal>(`${root}/global`, { method: 'PUT', body }),
  configurations: () => request<{ items: XNodeConfiguration[] }>(`${root}/configurations`, { cache: 'no-store' }),
  saveConfiguration: (body: Partial<XNodeConfiguration>) => request<XNodeConfiguration>(`${root}/configurations${body.id ? `/${encodeURIComponent(body.id)}` : ''}`, { method: body.id ? 'PUT' : 'POST', body }),
  validate: (document: object) => request<XNodeValidation>(`${root}/configurations/validate`, { method: 'POST', body: { document } }),
  snippets: () => request<{ items: XNodeSnippet[] }>(`${root}/snippets`, { cache: 'no-store' }),
  saveSnippet: (body: Partial<XNodeSnippet>) => request<XNodeSnippet>(`${root}/snippets${body.id ? `/${encodeURIComponent(body.id)}` : ''}`, { method: body.id ? 'PUT' : 'POST', body }),
  groups: () => request<{ items: XNodeGroup[] }>(`${root}/groups`, { cache: 'no-store' }),
  saveGroup: (body: Partial<XNodeGroup>) => request<XNodeGroup>(`${root}/groups${body.id ? `/${encodeURIComponent(body.id)}` : ''}`, { method: body.id ? 'PUT' : 'POST', body }),
  endpoints: () => request<{ items: XNodeEndpoint[] }>(`${root}/endpoints`, { cache: 'no-store' }),
  saveEndpoint: (body: Partial<XNodeEndpoint>) => request<XNodeEndpoint>(`${root}/endpoints${body.id ? `/${encodeURIComponent(body.id)}` : ''}`, { method: body.id ? 'PUT' : 'POST', body }),
  metrics: () => request<XNodeMetrics>(`${root}/metrics`, { cache: 'no-store' }),
  command: (id: string, body: XNodeCommand) => request<Schema['OperationReceipt']>(`${root}/nodes/${encodeURIComponent(id)}/commands`, { method: 'POST', body, headers: { 'Idempotency-Key': crypto.randomUUID() }, cache: 'no-store' }),
  migration: () => request<XNodeMigration>(`${root}/migration`, { cache: 'no-store' }),
  previewMigration: (body: XNodeMigrationPreviewWrite) => request<XNodeMigrationPreview>(`${root}/migration/preview`, { method: 'POST', body, cache: 'no-store' }),
  importMigration: (digest: string) => request<Schema['OperationReceipt']>(`${root}/migration/import`, { method: 'POST', body: { digest }, headers: { 'Idempotency-Key': crypto.randomUUID() }, cache: 'no-store' }),
  stageFinalMigration: (sourceBackupReference: string) => request<Schema['OperationReceipt']>(`${root}/migration/final-export`, { method: 'POST', body: { sourceBackupReference }, headers: { 'Idempotency-Key': crypto.randomUUID() }, cache: 'no-store' }),
  finalizeMigration: (digest: string) => request<Schema['OperationReceipt']>(`${root}/migration/finalize`, { method: 'POST', body: { digest }, headers: { 'Idempotency-Key': crypto.randomUUID() }, cache: 'no-store' }),
  migrateNode: (id: string, action: 'cutover' | 'rollback') => request<Schema['OperationReceipt']>(`${root}/migration/nodes/${encodeURIComponent(id)}/${action}`, { method: 'POST', headers: { 'Idempotency-Key': crypto.randomUUID() }, cache: 'no-store' }),
}
