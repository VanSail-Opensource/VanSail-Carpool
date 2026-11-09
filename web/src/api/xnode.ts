import type { components } from './generated'
import { request } from './http'

type Schema = components['schemas']
export type NamedService = Schema['XNodeNamedService']
export type NamedServiceWrite = Schema['XNodeNamedServiceWrite']
export type XNodeCustomization = Schema['XNodeCustomization']
export type XNode = Schema['XNodeNode']
export type XNodeWrite = Schema['XNodeNodeWrite']
export type XNodeSettings = Schema['XNodeSettings']
export type DDNSHostBinding = NonNullable<XNodeSettings['ddnsBindings']>[string]
export type XNodeGlobal = Schema['XNodeGlobal']
export type XNodeConfiguration = Schema['XNodeConfiguration']
export type XNodeSnippet = Schema['XNodeSnippet']
export type XNodeGroup = Schema['XNodeGroup']
export type XNodeEndpoint = Schema['XNodeEndpoint']
export type XNodeEndpointDraft = Schema['XNodeEndpointDraft']
export type XNodeEndpointField = Schema['XNodeEndpointField']
export type XNodeEndpointPreview = Schema['XNodeEndpointPreview']
export type XNodeMetrics = Schema['XNodeMetricsSummary']
export type XNodeValidation = Schema['XNodeValidation']
export type XNodeEnrollment = Schema['XNodeEnrollment']
export type XNodeCommand = Schema['XNodeCommandWrite']
export type SubscriptionTemplate = Schema['XNodeSubscriptionTemplate']
export type SubscriptionTemplateConfig = Schema['XNodeSubscriptionTemplateConfig']

const root = '/api/v1/admin/xnode'
export const xnodeApi = {
  providerZones: (id: string, signal?: AbortSignal) => request<Schema['XNodeProviderZones']>(`${root}/providers/${encodeURIComponent(id)}/zones`, { cache: 'no-store', signal }),
  routingTemplates: (signal?: AbortSignal) => request<Schema['XNodeRoutingTemplateConfig']>(`${root}/routing-templates`, { cache: 'no-store', signal }),
  saveRoutingTemplates: (body: Schema['XNodeRoutingTemplateConfig']) => request<Schema['XNodeRoutingTemplateConfig']>(`${root}/routing-templates`, { method: 'PUT', body, cache: 'no-store' }),
  namedServices: () => request<{ items: NamedService[] }>(`${root}/service-identities`, { cache: 'no-store' }),
  saveNamedService: (id: string | undefined, body: NamedServiceWrite) => request<NamedService>(`${root}/service-identities${id ? `/${encodeURIComponent(id)}` : ''}`, { method: id ? 'PUT' : 'POST', body, cache: 'no-store' }),
  serviceSubscription: (id: string) => request<Schema['XNodeSubscription']>(`${root}/service-identities/${encodeURIComponent(id)}/subscription`, { cache: 'no-store' }),
  rotateServiceSubscription: (id: string) => request<{ url: string }>(`${root}/service-identities/${encodeURIComponent(id)}/subscription/rotate`, { method: 'POST', cache: 'no-store' }),
  serviceRouting: (id: string, node: string) => request<XNodeCustomization>(`${root}/service-identities/${encodeURIComponent(id)}/routing/${encodeURIComponent(node)}`, { cache: 'no-store' }),
  saveServiceRouting: (id: string, node: string, part: 'outbounds' | 'rules', body: XNodeCustomization) => request<XNodeCustomization>(`${root}/service-identities/${encodeURIComponent(id)}/routing/${encodeURIComponent(node)}`, { method: 'PUT', body, query: { part }, cache: 'no-store' }),
  subscriptionTemplates: (signal?: AbortSignal) => request<SubscriptionTemplateConfig>(`${root}/subscription-templates`, { cache: 'no-store', signal }),
  saveSubscriptionTemplates: (body: SubscriptionTemplateConfig) => request<SubscriptionTemplateConfig>(`${root}/subscription-templates`, { method: 'PUT', body, cache: 'no-store' }),
  previewSubscriptionTemplate: (body: SubscriptionTemplate, signal?: AbortSignal) => request<Schema['XNodeSubscriptionTemplatePreview']>(`${root}/subscription-templates/preview`, { method: 'POST', body, cache: 'no-store', signal }),
  previewSubscriptionSelection: (body: { config: SubscriptionTemplateConfig; format: string; userAgent: string }, signal?: AbortSignal) => request<{ validation: XNodeValidation; selection: { format: string; templateId: string; source: string; responseKind?: string; statusCode?: number } }>(`${root}/subscription-templates/selection-preview`, { method: 'POST', body, cache: 'no-store', signal }),
  nodes: (signal?: AbortSignal) => request<{ items: XNode[] }>(`${root}/nodes`, { cache: 'no-store', signal }),
  node: (id: string, signal?: AbortSignal) => request<Schema['XNodeNodeDetail']>(`${root}/nodes/${encodeURIComponent(id)}`, { cache: 'no-store', signal }),
  saveNode: (node: Partial<XNodeWrite>, settings: XNodeSettings) => request<XNode>(`${root}/nodes${node.id ? `/${encodeURIComponent(node.id)}` : ''}`, { method: node.id ? 'PUT' : 'POST', body: { node, settings } }),
  removeNode: (id: string, key: string) => request<Schema['OperationReceipt']>(`${root}/nodes/${encodeURIComponent(id)}/removal`, { method: 'POST', headers: { 'Idempotency-Key': key }, cache: 'no-store' }),
  enroll: (id: string) => request<XNodeEnrollment>(`${root}/nodes/${encodeURIComponent(id)}/enrollment`, { method: 'POST', cache: 'no-store' }),
  global: (signal?: AbortSignal) => request<XNodeGlobal>(`${root}/global`, { cache: 'no-store', signal }),
  saveGlobal: (body: XNodeGlobal, part?: 'automaticWhitelist') => request<XNodeGlobal>(`${root}/global`, { method: 'PUT', body, query: { part } }),
  configurations: (signal?: AbortSignal) => request<{ items: XNodeConfiguration[] }>(`${root}/configurations`, { cache: 'no-store', signal }),
  saveConfiguration: (body: Partial<XNodeConfiguration>) => request<XNodeConfiguration>(`${root}/configurations${body.id ? `/${encodeURIComponent(body.id)}` : ''}`, { method: body.id ? 'PUT' : 'POST', body }),
  validate: (document: object, snippetIds: string[] = []) => request<XNodeValidation>(`${root}/configurations/validate`, { method: 'POST', body: { document, snippetIds } }),
  snippets: (signal?: AbortSignal) => request<{ items: XNodeSnippet[] }>(`${root}/snippets`, { cache: 'no-store', signal }),
  saveSnippet: (body: Partial<XNodeSnippet>) => request<XNodeSnippet>(`${root}/snippets${body.id ? `/${encodeURIComponent(body.id)}` : ''}`, { method: body.id ? 'PUT' : 'POST', body }),
  groups: (signal?: AbortSignal) => request<{ items: XNodeGroup[] }>(`${root}/groups`, { cache: 'no-store', signal }),
  deleteGroup: (id: string) => request<void>(`${root}/groups/${encodeURIComponent(id)}`, { method: 'DELETE', cache: 'no-store' }),
  saveGroup: (body: Partial<XNodeGroup>) => request<XNodeGroup>(`${root}/groups${body.id ? `/${encodeURIComponent(body.id)}` : ''}`, { method: body.id ? 'PUT' : 'POST', body }),
  endpoints: (signal?: AbortSignal) => request<{ items: XNodeEndpoint[] }>(`${root}/endpoints`, { cache: 'no-store', signal }),
	deleteEndpoint: (id: string) => request<void>(`${root}/endpoints/${encodeURIComponent(id)}`, { method: 'DELETE', cache: 'no-store' }),
  saveEndpoint: (body: Schema['XNodeEndpointDraft']) => request<XNodeEndpoint>(`${root}/endpoints${body.id ? `/${encodeURIComponent(body.id)}` : ''}`, { method: body.id ? 'PUT' : 'POST', body }),
  previewEndpoint: (body: XNodeEndpointDraft, signal?: AbortSignal) => request<XNodeEndpointPreview>(`${root}/endpoints/preview`, { method: 'POST', body, signal, cache: 'no-store' }),
  metrics: (signal?: AbortSignal) => request<XNodeMetrics>(`${root}/metrics?summary=true`, { cache: 'no-store', signal }),
  logs: (id: string, signal?: AbortSignal) => request<Schema['XNodeLogs']>(`${root}/nodes/${encodeURIComponent(id)}/logs`, { cache: 'no-store', signal }),
  command: (id: string, body: XNodeCommand, key: string = crypto.randomUUID()) => request<Schema['OperationReceipt']>(`${root}/nodes/${encodeURIComponent(id)}/commands`, { method: 'POST', body, headers: { 'Idempotency-Key': key }, cache: 'no-store' }),
  commandResult: (id: string) => request<Schema['XNodeCommandResult']>(`${root}/operations/${encodeURIComponent(id)}`, { cache: 'no-store' }),
  certificateOperation: (id: string, profileId: string, signal?: AbortSignal) => request<{ receipt: Schema['OperationReceipt'] | null; result: Record<string, unknown> }>(`${root}/nodes/${encodeURIComponent(id)}/commands?profileId=${encodeURIComponent(profileId)}`, { cache: 'no-store', signal }),
}
