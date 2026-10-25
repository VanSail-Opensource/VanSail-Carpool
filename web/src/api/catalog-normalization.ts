import type { Catalog, CatalogNode, Combo, PurchaseQuote, ProxyNode, SquadProduct } from './types'

function objectArray<T>(value: unknown): T[] {
  if (!Array.isArray(value)) return []
  return value.filter((item): item is T => typeof item === 'object' && item !== null)
}

function normalizeSquadProduct(value: SquadProduct): SquadProduct {
  const source = value as unknown as { accessibleNodes?: unknown }
  return {
    ...value,
    accessibleNodes: objectArray<CatalogNode>(source.accessibleNodes),
  }
}

function normalizeCombo(value: Combo): Combo {
  const source = value as unknown as { includedSquads?: unknown }
  return {
    ...value,
    includedSquads: objectArray<SquadProduct>(source.includedSquads).map(normalizeSquadProduct),
  }
}

export function normalizeCatalog(value: Catalog | null | undefined): Catalog {
  const source = value && typeof value === 'object'
    ? value as unknown as { combos?: unknown; addons?: unknown; nodes?: unknown }
    : {}

  return {
    autoRenewalBlockedUntil: value?.autoRenewalBlockedUntil ?? null,
    combos: objectArray<Combo>(source.combos).map(normalizeCombo),
    addons: objectArray<SquadProduct>(source.addons).map(normalizeSquadProduct),
    nodes: objectArray<CatalogNode>(source.nodes),
  }
}

export function normalizePurchaseQuote(value: PurchaseQuote): PurchaseQuote {
  const source = value as unknown as { accessibleNodes?: unknown }
  return {
    ...value,
    accessibleNodes: objectArray<ProxyNode>(source.accessibleNodes),
  }
}
