import type { SubscriptionTemplate } from '@/api/xnode'

export const templateFormats = ['xray', 'sing-box', 'clash', 'stash'] as const
export const templateKinds = [...templateFormats, 'empty-http'] as const
export const subscriptionFormats = ['base64', 'native', ...templateFormats] as const
export function subscriptionPreset(format: SubscriptionTemplate['format']): Record<string, unknown> {
  if (format === 'xray') return { inbounds: [], outbounds: [{ SubscriptionProxies: {} }, { tag: 'direct', protocol: 'freedom' }], routing: { rules: [] } }
  if (format === 'sing-box') return { outbounds: [{ type: 'selector', tag: 'proxy', outbounds: [{ SubscriptionNames: {} }] }, { SubscriptionProxies: {} }, { type: 'direct', tag: 'direct' }], route: { final: 'proxy', auto_detect_interface: true } }
  return { proxies: [{ SubscriptionProxies: {} }], 'proxy-groups': [{ name: 'proxy', type: 'select', proxies: [{ SubscriptionNames: {} }] }], rules: ['MATCH,proxy'] }
}
