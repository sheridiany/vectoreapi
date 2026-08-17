import { useTranslation } from 'react-i18next'
import { Area, AreaChart, CartesianGrid, XAxis, YAxis } from 'recharts'

import {
  ChartContainer,
  ChartTooltip,
  ChartTooltipContent,
  type ChartConfig,
} from '@/components/ui/chart'
import { toIntlLocale } from '@/i18n/languages'
import { formatNumber } from '@/lib/format'

import type { EnterpriseUsageDaily } from '../types'

export function EnterpriseUsageChart(props: { data: EnterpriseUsageDaily[] }) {
  const { i18n, t } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage ?? i18n.language)
  const chartConfig = {
    net_quota: {
      label: t('Net consumption'),
      color: 'var(--chart-1)',
    },
  } satisfies ChartConfig
  const chartData = props.data.map((item) => ({
    label: new Intl.DateTimeFormat(locale, {
      month: 'short',
      day: 'numeric',
    }).format(new Date(item.start_at * 1000)),
    net_quota: item.net_quota,
  }))

  return (
    <ChartContainer config={chartConfig} className='h-[280px] w-full'>
      <AreaChart data={chartData} margin={{ left: 8, right: 12, top: 10 }}>
        <CartesianGrid vertical={false} />
        <XAxis
          dataKey='label'
          tickLine={false}
          axisLine={false}
          tickMargin={8}
          minTickGap={24}
        />
        <YAxis
          tickLine={false}
          axisLine={false}
          tickMargin={8}
          tickFormatter={(value: number) => formatNumber(value)}
          width={72}
        />
        <ChartTooltip
          cursor={false}
          content={<ChartTooltipContent indicator='line' />}
        />
        <Area
          dataKey='net_quota'
          type='monotone'
          fill='var(--color-net_quota)'
          fillOpacity={0.16}
          stroke='var(--color-net_quota)'
          strokeWidth={2}
          dot={false}
        />
      </AreaChart>
    </ChartContainer>
  )
}
