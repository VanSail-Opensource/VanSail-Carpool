import { describe, expect, it } from 'vitest'

import type { components, paths } from './generated'

type IsRequired<T, Key extends keyof T> = object extends Pick<T, Key> ? false : true
type Squad = components['schemas']['SquadProduct']
type Node = components['schemas']['CatalogNode']

describe('generated OpenAPI release contract', () => {
  it('keeps retired reset automation absent and nested nodes required', () => {
    const contractChecks: [
      '/api/v1/me/traffic-reset-automation' extends keyof paths ? false : true,
      IsRequired<Squad, 'accessibleNodes'>,
      null extends Node['providerName'] ? true : false,
    ] = [true, true, true]

    expect(contractChecks).toEqual([true, true, true])
  })
})
