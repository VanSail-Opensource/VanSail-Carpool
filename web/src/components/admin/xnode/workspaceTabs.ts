export const workspaceTabs = ['nodes', 'global', 'configurations', 'templates', 'routing', 'services', 'groups', 'firewall', 'leases', 'metrics'] as const
export type WorkspaceTab = typeof workspaceTabs[number]
export type WorkspaceResource = 'nodes' | 'configurations' | 'snippets' | 'groups' | 'endpoints'

const resources: Record<WorkspaceTab, readonly WorkspaceResource[]> = {
  nodes: ['nodes', 'configurations', 'endpoints'],
  configurations: ['configurations', 'snippets'],
  groups: ['nodes', 'configurations', 'groups', 'endpoints'],
  services: ['nodes', 'groups'],
  leases: ['nodes'],
  metrics: ['nodes'],
  global: [], templates: [], routing: [], firewall: [],
}

export function workspaceTab(value: unknown): WorkspaceTab {
  return typeof value === 'string' && workspaceTabs.includes(value as WorkspaceTab) ? value as WorkspaceTab : 'nodes'
}

export const workspaceResources = (tab: WorkspaceTab) => resources[tab]
export const workspaceNeedsMetrics = (tab: WorkspaceTab) => tab === 'nodes' || tab === 'metrics'
