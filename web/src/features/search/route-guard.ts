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
import { redirect } from '@tanstack/react-router'

import { ROLE } from '@/lib/roles'
import { useAuthStore, type AuthUser } from '@/stores/auth-store'

export function canManageSearchAgentKeys(user: AuthUser | null | undefined) {
  return Boolean(
    user &&
    (user.role === ROLE.SUPER_ADMIN ||
      user.enterprise?.role === 'owner' ||
      user.enterprise?.role === 'admin')
  )
}

export function canManageSearch(user: AuthUser | null | undefined) {
  return user?.role === ROLE.SUPER_ADMIN
}

export function canViewSearchUsage(user: AuthUser | null | undefined) {
  return user?.role === ROLE.SUPER_ADMIN
}

export function requireSearchAdmin() {
  const user = useAuthStore.getState().auth.user
  if (!user || !canManageSearch(user)) {
    throw redirect({ to: '/403' })
  }
  return user
}

export function requireSearchAgentKeyAdmin() {
  const user = useAuthStore.getState().auth.user
  if (!user || !canManageSearchAgentKeys(user)) {
    throw redirect({ to: '/403' })
  }
  return user
}

export function requireSearchPlatformAdmin() {
  const user = useAuthStore.getState().auth.user
  if (!user || !canViewSearchUsage(user)) {
    throw redirect({ to: '/403' })
  }
  return user
}
