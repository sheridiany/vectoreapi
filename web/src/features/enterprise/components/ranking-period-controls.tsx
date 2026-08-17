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
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'

import { getCustomRankingRange } from '../lib/ranking-range'
import type { EnterpriseRankingPeriod } from '../types'

const periods: { id: EnterpriseRankingPeriod; labelKey: string }[] = [
  { id: 'today', labelKey: 'Today' },
  { id: 'week', labelKey: 'This week' },
  { id: 'month', labelKey: 'This month' },
  { id: 'custom', labelKey: 'Custom range' },
]

export function RankingPeriodControls(props: {
  period: EnterpriseRankingPeriod
  customStart: string
  customEnd: string
  onPeriodChange: (period: EnterpriseRankingPeriod) => void
  onCustomStartChange: (value: string) => void
  onCustomEndChange: (value: string) => void
}) {
  const { t } = useTranslation()
  const customRangeInvalid =
    props.period === 'custom' &&
    Boolean(props.customStart && props.customEnd) &&
    !getCustomRankingRange(props.customStart, props.customEnd)

  return (
    <div className='space-y-2'>
      <div className='flex flex-wrap items-center gap-2'>
        {periods.map((item) => (
          <Button
            key={item.id}
            type='button'
            size='sm'
            variant={props.period === item.id ? 'default' : 'outline'}
            aria-pressed={props.period === item.id}
            onClick={() => props.onPeriodChange(item.id)}
          >
            {t(item.labelKey)}
          </Button>
        ))}
        {props.period === 'custom' && (
          <>
            <Input
              aria-label={t('Start date')}
              type='date'
              value={props.customStart}
              onChange={(event) =>
                props.onCustomStartChange(event.target.value)
              }
              className='h-9 w-auto'
            />
            <Input
              aria-label={t('End date')}
              type='date'
              value={props.customEnd}
              onChange={(event) => props.onCustomEndChange(event.target.value)}
              className='h-9 w-auto'
            />
          </>
        )}
      </div>
      {customRangeInvalid && (
        <p className='text-destructive text-sm'>
          {t('End date must be on or after start date.')}
        </p>
      )}
    </div>
  )
}
