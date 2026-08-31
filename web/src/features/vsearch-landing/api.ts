/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { api } from '@/lib/api'

import type { PublicSearchCatalogItem } from './types'

export async function fetchPublicSearchCatalog(): Promise<
  PublicSearchCatalogItem[]
> {
  const response = await api.get<{
    success: boolean
    message?: string
    data?: PublicSearchCatalogItem[]
  }>('/api/search/public/catalog', {
    skipBusinessError: true,
    skipErrorHandler: true,
  })
  if (!response.data.success || !response.data.data) {
    throw new Error(
      response.data.message || 'Failed to load public capability catalog'
    )
  }
  return response.data.data
}
