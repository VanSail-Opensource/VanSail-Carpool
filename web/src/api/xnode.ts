import type { components } from './generated'
import { request } from './http'

type Schema = components['schemas']
export type NamedService = Schema['XNodeNamedService']
export type NamedServiceWrite = Schema['XNodeNamedServiceWrite']
export type XNodeCustomization = Schema['XNodeCustomization']
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
export type SubscriptionTemplate = Schema['XNodeSubscriptionTemplate']
export type SubscriptionTemplateConfig = Schema['XNodeSubscriptionTemplateConfig']

const root = '/api/v1/admin/xnode'
export const xnodeApi = {
  namedServices: () => request<{ items: NamedService[] }>(`${root}/service-identities`, { cache: 'no-store' }),
  saveNamedService: (id: string | undefined, body: NamedServiceWrite) => request<NamedService>(`${root}/service-identities${id ? `/${encodeURIComponent(id)}` : ''}`, { method: id ? 'PUT' : 'POST', body, cache: 'no-store' }),
  serviceSubscription: (id: string) => request<Schema['XNodeSubscription']>(`${root}/service-identities/${encodeURIComponent(id)}/subscription`, { cache: 'no-store' }),
  rotateServiceSubscription: (id: string) => request<{ url: string }>(`${root}/service-identities/${encodeURIComponent(id)}/subscription/rotate`, { method: 'POST', cache: 'no-store' }),
  serviceRouting: (id: string, node: string) => request<XNodeCustomization>(`${root}/service-identities/${encodeURIComponent(id)}/routing/${encodeURIComponent(node)}`, { cache: 'no-store' }),
  saveServiceRouting: (id: string, node: string, part: 'outbounds' | 'rules', body: XNodeCustomization) => request<XNodeCustomization>(`${root}/service-identities/${encodeURIComponent(id)}/routing/${encodeURIComponent(node)}`, { method: 'PUT', body, query: { part }, cache: 'no-store' }),
  subscriptionTemplates: () => request<SubscriptionTemplateConfig>(`${root}/subscription-templates`, { cache: 'no-store' }),
  saveSubscriptionTemplates: (body: SubscriptionTemplateConfig) => request<SubscriptionTemplateConfig>(`${root}/subscription-templates`, { method: 'PUT', body, cache: 'no-store' }),
  previewSubscriptionTemplate: (body: SubscriptionTemplate) => request<Schema['XNodeSubscriptionTemplatePreview']>(`${root}/subscription-templates/preview`, { method: 'POST', body, cache: 'no-store' }),
  nodes: () => request<{ items: XNode[] }>(`${root}/nodes`, { cache: 'no-store' }),
  node: (id: string) => request<Schema['XNodeNodeDetail']>(`${root}/nodes/${encodeURIComponent(id)}`, { cache: 'no-store' }),
  saveNode: (node: Partial<XNode>, settings: XNodeSettings) => request<XNode>(`${root}/nodes${node.id ? `/${encodeURIComponent(node.id)}` : ''}`, { method: node.id ? 'PUT' : 'POST', body: { node, settings } }),
  enroll: (id: string) => request<XNodeEnrollment>(`${root}/nodes/${encodeURIComponent(id)}/enrollment`, { method: 'POST', cache: 'no-store' }),
  global: () => request<XNodeGlobal>(`${root}/global`, { cache: 'no-store' }),
  saveGlobal: (body: XNodeGlobal) => request<XNodeGlobal>(`${root}/global`, { method: 'PUT', body }),
  configurations: () => request<{ items: XNodeConfiguration[] }>(`${root}/configurations`, { cache: 'no-store' }),
  saveConfiguration: (body: Partial<XNodeConfiguration>) => request<XNodeConfiguration>(`${root}/configurations${body.id ? `/${encodeURIComponent(body.id)}` : ''}`, { method: body.id ? 'PUT' : 'POST', body }),
  validate: (document: object, snippetIds: string[] = []) => request<XNodeValidation>(`${root}/configurations/validate`, { method: 'POST', body: { document, snippetIds } }),
  snippets: () => request<{ items: XNodeSnippet[] }>(`${root}/snippets`, { cache: 'no-store' }),
  saveSnippet: (body: Partial<XNodeSnippet>) => request<XNodeSnippet>(`${root}/snippets${body.id ? `/${encodeURIComponent(body.id)}` : ''}`, { method: body.id ? 'PUT' : 'POST', body }),
  groups: () => request<{ items: XNodeGroup[] }>(`${root}/groups`, { cache: 'no-store' }),
  saveGroup: (body: Partial<XNodeGroup>) => request<XNodeGroup>(`${root}/groups${body.id ? `/${encodeURIComponent(body.id)}` : ''}`, { method: body.id ? 'PUT' : 'POST', body }),
  endpoints: () => request<{ items: XNodeEndpoint[] }>(`${root}/endpoints`, { cache: 'no-store' }),
  saveEndpoint: (body: Partial<XNodeEndpoint>) => request<XNodeEndpoint>(`${root}/endpoints${body.id ? `/${encodeURIComponent(body.id)}` : ''}`, { method: body.id ? 'PUT' : 'POST', body }),
  metrics: () => request<XNodeMetrics>(`${root}/metrics`, { cache: 'no-store' }),
  command: (id: string, body: XNodeCommand, key: string = crypto.randomUUID()) => request<Schema['OperationReceipt']>(`${root}/nodes/${encodeURIComponent(id)}/commands`, { method: 'POST', body, headers: { 'Idempotency-Key': key }, cache: 'no-store' }),
  commandResult: (id: string) => request<Schema['XNodeCommandResult']>(`${root}/operations/${encodeURIComponent(id)}`, { cache: 'no-store' }),
  migration: () => request<XNodeMigration>(`${root}/migration`, { cache: 'no-store' }),
  previewMigration: (body: XNodeMigrationPreviewWrite) => request<XNodeMigrationPreview>(`${root}/migration/preview`, { method: 'POST', body, cache: 'no-store' }),
  importMigration: (digest: string) => request<Schema['OperationReceipt']>(`${root}/migration/import`, { method: 'POST', body: { digest }, headers: { 'Idempotency-Key': crypto.randomUUID() }, cache: 'no-store' }),
  stageFinalMigration: (sourceBackupReference: string) => request<Schema['OperationReceipt']>(`${root}/migration/final-export`, { method: 'POST', body: { sourceBackupReference }, headers: { 'Idempotency-Key': crypto.randomUUID() }, cache: 'no-store' }),
  finalizeMigration: (digest: string) => request<Schema['OperationReceipt']>(`${root}/migration/finalize`, { method: 'POST', body: { digest }, headers: { 'Idempotency-Key': crypto.randomUUID() }, cache: 'no-store' }),
  migrateNode: (id: string, action: 'cutover' | 'rollback') => request<Schema['OperationReceipt']>(`${root}/migration/nodes/${encodeURIComponent(id)}/${action}`, { method: 'POST', headers: { 'Idempotency-Key': crypto.randomUUID() }, cache: 'no-store' }),
}
