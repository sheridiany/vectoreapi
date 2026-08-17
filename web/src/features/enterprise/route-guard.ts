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

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { redirect } from '@tanstack/react-router'

import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'

export function getCurrentEnterpriseUser() {
  return useAuthStore.getState().auth.user
}

export function requireEnterpriseMember() {
  const user = getCurrentEnterpriseUser()
  if (!user) {
    throw redirect({ to: '/403' })
  }

  if (user.role === ROLE.SUPER_ADMIN) {
    throw redirect({ to: '/enterprise/admin' })
  }

  if (!user.enterprise) {
    throw redirect({ to: '/403' })
  }

  return user
}

export function requireEnterpriseManager() {
  const user = requireEnterpriseMember()
  const canManage =
    user.role === ROLE.SUPER_ADMIN ||
    user.enterprise?.role === 'owner' ||
    user.enterprise?.role === 'admin'

  if (!canManage) {
    throw redirect({ to: '/403' })
  }

  return user
}

export function requireEnterpriseViewer() {
  const user = requireEnterpriseMember()
  const canView =
    user.role === ROLE.SUPER_ADMIN ||
    user.enterprise?.role === 'owner' ||
    user.enterprise?.role === 'admin' ||
    user.enterprise?.role === 'auditor'

  if (!canView) {
    throw redirect({ to: '/403' })
  }

  return user
}

export function requirePlatformEnterpriseAdmin() {
  const user = getCurrentEnterpriseUser()
  if (!user || user.role !== ROLE.SUPER_ADMIN) {
    throw redirect({ to: '/403' })
  }

  return user
}
