import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
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
*/
import { Link } from '@tanstack/react-router'
import {
  AlertTriangle,
  Check,
  ExternalLink,
  KeyRound,
  RotateCcw,
  Route,
  ShieldCheck,
  WandSparkles,
} from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { CopyButton } from '@/components/copy-button'
import { ErrorState } from '@/components/error-state'
import { LoadingState } from '@/components/loading-state'
import { StatusBadge } from '@/components/status-badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { TitledCard } from '@/components/ui/titled-card'
import { ApiInfoPanel } from '@/features/dashboard/components/overview/api-info-panel'
import { formatNumber, formatTimestamp } from '@/lib/format'
import { useAuthStore } from '@/stores/auth-store'

import {
  applyEnterpriseKeyPolicy,
  getEnterpriseKeyPolicy,
  rollbackEnterpriseKeyPolicy,
} from '../api'
import { EnterpriseShell } from './enterprise-shell'

export function EnterpriseApiAccessPage() {
  const { t } = useTranslation()
  const user = useAuthStore((state) => state.auth.user)
  const queryClient = useQueryClient()
  const enterpriseId = user?.enterprise?.id
  const isEnterpriseManager =
    user?.enterprise?.role === 'owner' || user?.enterprise?.role === 'admin'
  const [applyDialogOpen, setApplyDialogOpen] = useState(false)
  const serverOrigin =
    import.meta.env.VITE_REACT_APP_SERVER_URL ||
    globalThis.location?.origin ||
    ''
  const apiBaseUrl = `${serverOrigin.replace(/\/$/, '')}/v1`
  const policyQuery = useQuery({
    queryKey: ['enterprise', enterpriseId, 'key-policy'],
    queryFn: async () => {
      if (!enterpriseId) return null
      const response = await getEnterpriseKeyPolicy(enterpriseId)
      if (!response.success || !response.data) {
        throw new Error(
          response.message || t('Unable to load enterprise key policy')
        )
      }
      return response.data
    },
    enabled: Boolean(enterpriseId),
  })
  const applyMutation = useMutation({
    mutationFn: async () => {
      if (!enterpriseId) throw new Error(t('No enterprise selected'))
      const response = await applyEnterpriseKeyPolicy(enterpriseId)
      if (!response.success || !response.data) {
        throw new Error(
          response.message || t('Failed to apply Auto key policy')
        )
      }
      return response.data
    },
    onSuccess: async (operation) => {
      setApplyDialogOpen(false)
      toast.success(
        t('Auto key policy applied to {{count}} keys', {
          count: operation.changed_count,
        })
      )
      await queryClient.invalidateQueries({
        queryKey: ['enterprise', enterpriseId, 'key-policy'],
      })
    },
    onError: (error) => toast.error(error.message),
  })
  const rollbackMutation = useMutation({
    mutationFn: async (operationId: number) => {
      if (!enterpriseId) throw new Error(t('No enterprise selected'))
      const response = await rollbackEnterpriseKeyPolicy(
        enterpriseId,
        operationId
      )
      if (!response.success || !response.data) {
        throw new Error(
          response.message || t('Failed to roll back Auto key policy')
        )
      }
      return response.data
    },
    onSuccess: async (operation) => {
      toast.success(
        t('Auto key policy rolled back for {{count}} keys', {
          count: operation.changed_count,
        })
      )
      await queryClient.invalidateQueries({
        queryKey: ['enterprise', enterpriseId, 'key-policy'],
      })
    },
    onError: (error) => toast.error(error.message),
  })

  return (
    <EnterpriseShell
      section='api-access'
      title={t('API access')}
      description={t(
        'Connect your applications to the enterprise gateway with the existing New API key workflow.'
      )}
    >
      <TitledCard
        title={t('Enterprise model access')}
        description={t(
          'Use one Auto key per member and inherit the gateway-wide model routing policy.'
        )}
        icon={<WandSparkles className='size-4' aria-hidden='true' />}
      >
        {policyQuery.isLoading && <LoadingState message={t('Loading...')} />}
        {policyQuery.isError && (
          <ErrorState
            title={t('Unable to load enterprise key policy')}
            description={t('Please try again later.')}
            onRetry={() => void policyQuery.refetch()}
          />
        )}
        {policyQuery.data && (
          <div className='space-y-5'>
            <div className='flex flex-wrap items-center justify-between gap-3'>
              <div className='flex items-center gap-2'>
                <StatusBadge
                  label={t('Auto enabled')}
                  icon={Check}
                  variant='success'
                  copyable={false}
                />
                <span className='text-muted-foreground text-sm'>
                  {t('All models follow the global Auto order')}
                </span>
              </div>
              <div className='flex items-center gap-2 text-xs'>
                <ShieldCheck
                  className='text-success size-4'
                  aria-hidden='true'
                />
                <span className='text-muted-foreground'>
                  {t('Secrets are never shown here')}
                </span>
              </div>
            </div>
            <div className='grid gap-3 sm:grid-cols-2 xl:grid-cols-4'>
              <PolicyMetric
                label={t('Active members')}
                value={formatNumber(policyQuery.data.active_member_count)}
              />
              <PolicyMetric
                label={t('Member keys')}
                value={formatNumber(policyQuery.data.total_key_count)}
              />
              <PolicyMetric
                label={t('Auto keys')}
                value={formatNumber(policyQuery.data.auto_key_count)}
              />
              <PolicyMetric
                label={t('Keys to convert')}
                value={formatNumber(policyQuery.data.convertible_key_count)}
                tone={policyQuery.data.convertible_key_count > 0}
              />
            </div>
            {policyQuery.data.convertible_key_count > 0 && (
              <div className='border-warning/40 bg-warning/10 text-warning flex gap-3 rounded-lg border p-3 text-sm'>
                <AlertTriangle
                  className='mt-0.5 size-4 shrink-0'
                  aria-hidden='true'
                />
                <p className='leading-5'>
                  {t(
                    'Applying the policy changes only non-Auto keys belonging to active enterprise members. Their previous group, retry setting and Auto order are saved for rollback.'
                  )}
                </p>
              </div>
            )}
            <div className='flex flex-wrap items-center gap-2'>
              {isEnterpriseManager && (
                <Button
                  onClick={() => setApplyDialogOpen(true)}
                  disabled={
                    policyQuery.data.convertible_key_count === 0 ||
                    applyMutation.isPending
                  }
                >
                  <WandSparkles aria-hidden='true' />
                  {t('Apply Auto to member keys')}
                </Button>
              )}
              {isEnterpriseManager &&
                policyQuery.data.last_operation?.status === 'applied' &&
                policyQuery.data.last_operation.changed_count > 0 && (
                  <Button
                    variant='outline'
                    onClick={() => {
                      const operation = policyQuery.data?.last_operation
                      if (operation) rollbackMutation.mutate(operation.id)
                    }}
                    disabled={rollbackMutation.isPending}
                  >
                    <RotateCcw aria-hidden='true' />
                    {t('Roll back latest change')}
                  </Button>
                )}
            </div>
            {policyQuery.data.last_operation && (
              <p className='text-muted-foreground text-xs'>
                {policyQuery.data.last_operation.status === 'applied'
                  ? t('Latest change: {{count}} keys at {{time}}', {
                      count: policyQuery.data.last_operation.changed_count,
                      time: formatTimestamp(
                        policyQuery.data.last_operation.created_at
                      ),
                    })
                  : t('Latest change was rolled back at {{time}}', {
                      time: formatTimestamp(
                        policyQuery.data.last_operation.rolled_back_at
                      ),
                    })}
              </p>
            )}
          </div>
        )}
      </TitledCard>

      <div className='grid gap-5 xl:grid-cols-[minmax(0,1.15fr)_minmax(320px,0.85fr)]'>
        <TitledCard
          title={t('Enterprise gateway')}
          description={t(
            'Use this base URL for OpenAI-compatible API requests.'
          )}
          icon={<Route className='size-4' aria-hidden='true' />}
        >
          <div className='space-y-5'>
            <div className='bg-muted/35 rounded-lg border p-4'>
              <div className='flex items-start justify-between gap-3'>
                <div className='min-w-0'>
                  <p className='text-muted-foreground text-xs font-medium'>
                    {t('API base URL')}
                  </p>
                  <p className='mt-1 truncate font-mono text-sm'>
                    {apiBaseUrl}
                  </p>
                </div>
                <CopyButton
                  value={apiBaseUrl}
                  variant='outline'
                  aria-label={t('Copy API base URL')}
                />
              </div>
            </div>
            <div className='grid gap-3 sm:grid-cols-2'>
              <Card>
                <CardContent className='p-4'>
                  <p className='text-muted-foreground text-xs font-medium'>
                    {t('Key ownership')}
                  </p>
                  <p className='mt-2 text-sm font-semibold'>
                    {t('Each member manages their own key')}
                  </p>
                </CardContent>
              </Card>
              <Card>
                <CardContent className='p-4'>
                  <p className='text-muted-foreground text-xs font-medium'>
                    {t('Model access')}
                  </p>
                  <p className='mt-2 text-sm font-semibold'>
                    {t('Use the Auto group when enabled')}
                  </p>
                </CardContent>
              </Card>
            </div>
            <div className='flex flex-wrap items-center gap-2'>
              <Button render={<Link to='/keys' />}>
                <KeyRound aria-hidden='true' />
                {t('Manage my API keys')}
              </Button>
              <Button
                variant='ghost'
                render={
                  <a
                    href='https://docs.newapi.pro/api/'
                    target='_blank'
                    rel='noreferrer'
                  />
                }
              >
                <ExternalLink aria-hidden='true' />
                {t('View API docs')}
              </Button>
            </div>
            <p className='text-muted-foreground text-xs leading-5'>
              {t(
                'For security, enterprise administrators cannot view or copy another member’s secret key. Key creation, rotation and deletion remain in the standard API Keys page.'
              )}
            </p>
          </div>
        </TitledCard>

        <ApiInfoPanel />
      </div>

      <Dialog open={applyDialogOpen} onOpenChange={setApplyDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('Apply Auto key policy?')}</DialogTitle>
            <DialogDescription>
              {t(
                'This will convert {{count}} non-Auto member keys to Auto. No secret key value will be read or exposed.',
                {
                  count: policyQuery.data?.convertible_key_count ?? 0,
                }
              )}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <DialogClose render={<Button variant='outline' />}>
              {t('Cancel')}
            </DialogClose>
            <Button
              onClick={() => applyMutation.mutate()}
              disabled={applyMutation.isPending}
            >
              {t('Confirm and apply')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </EnterpriseShell>
  )
}

function PolicyMetric(props: { label: string; value: string; tone?: boolean }) {
  return (
    <Card>
      <CardContent className='p-4'>
        <p className='text-muted-foreground text-xs font-medium'>
          {props.label}
        </p>
        <p
          className={
            props.tone
              ? 'text-warning mt-2 text-xl font-semibold'
              : 'mt-2 text-xl font-semibold'
          }
        >
          {props.value}
        </p>
      </CardContent>
    </Card>
  )
}
