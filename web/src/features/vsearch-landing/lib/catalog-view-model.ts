/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import type { PublicSearchCatalogItem } from '../types'

export type CatalogItemState =
  | 'available'
  | 'catalog'
  | 'unavailable'
  | 'disabled'

export interface CatalogGroup {
  category: string
  items: PublicSearchCatalogItem[]
}

export interface CatalogMetrics {
  capabilityCount: number
  availableCapabilityCount: number
  categoryCount: number
  interfaceCount: number
  callableInterfaceCount: number
  lastSyncedAt: number | null
}

export interface CatalogViewModel {
  groups: CatalogGroup[]
  metrics: CatalogMetrics
}

export function getCallableInterfaceCount(item: PublicSearchCatalogItem) {
  const interfaceCount = Math.max(0, item.interface_count)
  if (item.available_interface_count !== undefined) {
    return Math.min(interfaceCount, Math.max(0, item.available_interface_count))
  }
  if (item.status === 'available' && item.enabled) {
    return interfaceCount
  }
  return 0
}

export function getCatalogItemState(
  item: PublicSearchCatalogItem
): CatalogItemState {
  if (item.status === 'disabled') return 'disabled'
  if (item.status === 'catalog') return 'catalog'
  if (!item.enabled) return 'disabled'
  if (item.status === 'available' && getCallableInterfaceCount(item) > 0) {
    return 'available'
  }
  return 'unavailable'
}

export function buildCatalogViewModel(
  catalog: PublicSearchCatalogItem[]
): CatalogViewModel {
  const groups = new Map<string, PublicSearchCatalogItem[]>()
  let availableCapabilityCount = 0
  let interfaceCount = 0
  let callableInterfaceCount = 0
  let lastSyncedAt: number | null = null

  for (const item of catalog) {
    const category = item.category.trim() || 'Capability'
    const items = groups.get(category) || []
    items.push(item)
    groups.set(category, items)

    if (getCatalogItemState(item) === 'available') {
      availableCapabilityCount += 1
    }
    interfaceCount += Math.max(0, item.interface_count)
    callableInterfaceCount += getCallableInterfaceCount(item)

    if (
      item.last_synced_at &&
      (lastSyncedAt === null || item.last_synced_at > lastSyncedAt)
    ) {
      lastSyncedAt = item.last_synced_at
    }
  }

  return {
    groups: Array.from(groups, ([category, items]) => ({ category, items })),
    metrics: {
      capabilityCount: catalog.length,
      availableCapabilityCount,
      categoryCount: groups.size,
      interfaceCount,
      callableInterfaceCount,
      lastSyncedAt,
    },
  }
}
