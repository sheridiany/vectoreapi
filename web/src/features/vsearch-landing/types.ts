/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
export type PublicSearchCatalogStatus =
  | 'available'
  | 'unavailable'
  | 'catalog'
  | 'disabled'

export interface PublicSearchCatalogItem {
  id: string
  name: string
  category: string
  description: string
  status: PublicSearchCatalogStatus
  enabled: boolean
  interface_count: number
  available_interface_count?: number
  supported_platforms?: string[]
  cost_label?: string
  price_min_micros?: number
  price_max_micros?: number
  last_synced_at?: number | null
}
