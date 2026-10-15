import { describe, expect, it } from 'vitest'

import type { components, paths } from './generated'

type IsRequired<T, Key extends keyof T> = object extends Pick<T, Key> ? false : true
type AutomationPath = paths['/api/v1/me/traffic-reset-automation']
type Squad = components['schemas']['SquadProduct']
type Node = components['schemas']['CatalogNode']
type Snapshot = components['schemas']['StatisticsSnapshot']

describe('generated OpenAPI release contract', () => {
  it('keeps reset automation, nested nodes, and predicted rollover required', () => {
    const contractChecks: [
      AutomationPath extends { get: unknown, put: unknown } ? true : false,
      IsRequired<Squad, 'accessibleNodes'>,
      null extends Node['providerName'] ? true : false,
      IsRequired<Snapshot['remote'], 'predictedAverageRollover'>,
      'averageRollover' extends keyof Snapshot['database'] ? false : true,
    ] = [true, true, true, true, true]

    expect(contractChecks).toEqual([true, true, true, true, true])
  })
})
