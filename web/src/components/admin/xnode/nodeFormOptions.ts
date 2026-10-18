import type { XNode, XNodeConfiguration } from '@/api/xnode'
import { z } from 'zod'
import { t } from '@/i18n'

export function nodeMultiplierSchema() {
  return z.string().regex(/^(0|[1-9]\d{0,3})(\.\d{1,6})?$/, t('nodeForms.multiplierInvalid')).refine(value => Number(value) <= 1000, t('nodeForms.multiplierInvalid')).transform(Number)
}

export function nodeInboundItems(nodeId: string | undefined, nodes: XNode[], configurations: XNodeConfiguration[]) {
  const document = configurations.find(item => item.id === nodes.find(node => node.id === nodeId)?.configId)?.document
  if (!Array.isArray(document?.inbounds)) return []
  return document.inbounds.flatMap(item => item && typeof item === 'object' && typeof item.tag === 'string' && !['tunnel', 'dokodemo-door'].includes(item.protocol) ? [{ value: item.tag, label: item.tag }] : [])
}

export function objectOption(value: unknown): Record<string, unknown> {
  return value && typeof value === 'object' && !Array.isArray(value) ? value as Record<string, unknown> : {}
}
