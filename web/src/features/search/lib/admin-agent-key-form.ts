/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.
*/
import { z } from 'zod'

import { SEARCH_CAPABILITY_GROUPS } from '../types'

export const adminSearchAgentKeySchema = z.object({
  user_id: z.number().int().positive('Select a user'),
  name: z
    .string()
    .trim()
    .min(1, 'Key name is required')
    .max(64, 'Key name is too long'),
  scopes: z.array(z.string()).min(1, 'Select at least one capability'),
})

export type AdminSearchAgentKeyFormValues = z.infer<
  typeof adminSearchAgentKeySchema
>

export function getAdminSearchAgentKeyFormDefaults(
  userId = 0
): AdminSearchAgentKeyFormValues {
  return {
    user_id: userId,
    name: '',
    scopes: SEARCH_CAPABILITY_GROUPS.map((group) => group.id),
  }
}
