/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { describe, expect, test } from 'vitest'

import {
  buildCatalogViewModel,
  getCallableInterfaceCount,
  getCatalogItemState,
} from '../lib/catalog-view-model'
import type { PublicSearchCatalogItem } from '../types'

const catalog: PublicSearchCatalogItem[] = [
  {
    id: 'available',
    name: 'Available capability',
    category: 'Search',
    description: 'Callable through two published interfaces.',
    status: 'available',
    enabled: true,
    interface_count: 3,
    available_interface_count: 2,
    last_synced_at: 20,
  },
  {
    id: 'catalog',
    name: 'Catalog capability',
    category: 'Search',
    description: 'Published without a callable provider binding.',
    status: 'catalog',
    enabled: true,
    interface_count: 1,
    last_synced_at: 10,
  },
  {
    id: 'unavailable',
    name: 'Unavailable capability',
    category: 'Extraction',
    description: 'Enabled contract with no ready interface.',
    status: 'unavailable',
    enabled: true,
    interface_count: 2,
    available_interface_count: 0,
  },
  {
    id: 'disabled',
    name: 'Disabled capability',
    category: 'Extraction',
    description: 'Explicitly disabled contract.',
    status: 'disabled',
    enabled: false,
    interface_count: 4,
  },
]

describe('public catalog view model', () => {
  test('keeps the four published readiness states distinct', () => {
    expect(catalog.map(getCatalogItemState)).toEqual([
      'available',
      'catalog',
      'unavailable',
      'disabled',
    ])
  })

  test('reports only enabled callable interfaces and clamps invalid counts', () => {
    expect(getCallableInterfaceCount(catalog[0])).toBe(2)
    expect(
      getCallableInterfaceCount({
        ...catalog[0],
        interface_count: 1,
        available_interface_count: 9,
      })
    ).toBe(1)
    expect(
      getCallableInterfaceCount({
        ...catalog[0],
        interface_count: -3,
        available_interface_count: -1,
      })
    ).toBe(0)
  })

  test('groups real capabilities and derives the public index totals', () => {
    const viewModel = buildCatalogViewModel(catalog)

    expect(viewModel.groups.map((group) => group.category)).toEqual([
      'Search',
      'Extraction',
    ])
    expect(viewModel.groups.map((group) => group.items.length)).toEqual([2, 2])
    expect(viewModel.metrics).toEqual({
      capabilityCount: 4,
      availableCapabilityCount: 1,
      categoryCount: 2,
      interfaceCount: 10,
      callableInterfaceCount: 2,
      lastSyncedAt: 20,
    })
  })
})
