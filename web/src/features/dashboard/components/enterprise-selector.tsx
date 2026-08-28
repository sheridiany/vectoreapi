/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { Building06Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useQuery } from '@tanstack/react-query'
import { useEffect, useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { getAllEnterprises } from '@/features/enterprise/api'

const ALL_ENTERPRISES_VALUE = 'all'

interface DashboardEnterpriseSelectorProps {
  enterpriseId?: number
  onChange: (enterpriseId?: number) => void
}

export function DashboardEnterpriseSelector(
  props: DashboardEnterpriseSelectorProps
) {
  const { enterpriseId, onChange } = props
  const { t } = useTranslation()
  const enterprisesQuery = useQuery({
    queryKey: ['enterprise-admin', 'enterprises', 'all'],
    queryFn: async () => {
      const response = await getAllEnterprises()
      if (!response.success) {
        throw new Error(response.message || t('Unable to load enterprises'))
      }
      return response.data?.items ?? []
    },
    staleTime: 60_000,
  })
  const items = useMemo(
    () => [
      { value: ALL_ENTERPRISES_VALUE, label: t('All enterprises') },
      ...(enterprisesQuery.data ?? []).map((enterprise) => ({
        value: String(enterprise.id),
        label:
          enterprise.status === 1
            ? enterprise.name
            : `${enterprise.name} · ${t('Disabled')}`,
      })),
    ],
    [enterprisesQuery.data, t]
  )
  const selectedValue =
    enterpriseId === undefined ? ALL_ENTERPRISES_VALUE : String(enterpriseId)
  const selectedLabel =
    items.find((item) => item.value === selectedValue)?.label ?? selectedValue

  useEffect(() => {
    if (
      enterpriseId === undefined ||
      !enterprisesQuery.data ||
      enterprisesQuery.data.some((enterprise) => enterprise.id === enterpriseId)
    ) {
      return
    }

    toast.error(t('Selected enterprise is no longer available'))
    onChange(undefined)
  }, [enterpriseId, enterprisesQuery.data, onChange, t])

  return (
    <Select
      items={items}
      value={selectedValue}
      onValueChange={(value) => {
        if (value === null || value === ALL_ENTERPRISES_VALUE) {
          onChange(undefined)
          return
        }
        onChange(Number(value))
      }}
    >
      <SelectTrigger
        aria-label={t('Enterprise filter')}
        className='w-[180px] max-w-full min-w-0 sm:w-[220px]'
        disabled={enterprisesQuery.isLoading || enterprisesQuery.isError}
      >
        <HugeiconsIcon
          icon={Building06Icon}
          strokeWidth={2}
          aria-hidden='true'
        />
        <SelectValue>
          <span className='min-w-0 flex-1 truncate'>{selectedLabel}</span>
        </SelectValue>
      </SelectTrigger>
      <SelectContent align='end' alignItemWithTrigger={false}>
        <SelectGroup>
          {items.map((item) => (
            <SelectItem key={item.value} value={item.value}>
              <span className='block max-w-[18rem] truncate'>{item.label}</span>
            </SelectItem>
          ))}
        </SelectGroup>
      </SelectContent>
    </Select>
  )
}
