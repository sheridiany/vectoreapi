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
import { ROLE } from '@/lib/roles'
import type { AuthUser } from '@/stores/auth-store'

type EnterpriseUser = Pick<AuthUser, 'role'> & {
  enterprise?: Pick<NonNullable<AuthUser['enterprise']>, 'role'>
}

export function canManageEnterprise(user: EnterpriseUser | null) {
  return Boolean(
    user &&
    (user.role === ROLE.SUPER_ADMIN ||
      user.enterprise?.role === 'owner' ||
      user.enterprise?.role === 'admin')
  )
}

export function canViewEnterprise(user: EnterpriseUser | null) {
  return Boolean(
    user &&
    (user.role === ROLE.SUPER_ADMIN ||
      user.enterprise?.role === 'owner' ||
      user.enterprise?.role === 'admin' ||
      user.enterprise?.role === 'auditor')
  )
}

export function canAppointEnterpriseOwner(user: EnterpriseUser | null) {
  return user?.role === ROLE.SUPER_ADMIN
}
