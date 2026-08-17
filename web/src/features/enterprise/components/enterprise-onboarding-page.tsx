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
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Copy, KeyRound, Settings2 } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import {
  StaticDataTable,
  type StaticDataTableColumn,
} from '@/components/data-table'
import { EmptyState } from '@/components/empty-state'
import { ErrorState } from '@/components/error-state'
import { LoadingState } from '@/components/loading-state'
import { StatusBadge } from '@/components/status-badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { TitledCard } from '@/components/ui/titled-card'
import { useAuthStore } from '@/stores/auth-store'

import {
  createEnterpriseInvitation,
  getEnterprise,
  getEnterpriseInvitations,
  updateEnterpriseRegistration,
  updateEnterpriseInvitation,
} from '../api'
import type { EnterpriseInvitation } from '../types'
import { EnterpriseShell } from './enterprise-shell'

type RegistrationMode = 'open' | 'invite' | 'closed'

export function EnterpriseOnboardingPage() {
  const { t } = useTranslation()
  const user = useAuthStore((state) => state.auth.user)
  const enterpriseId = user?.enterprise?.id
  const queryClient = useQueryClient()
  const [mode, setMode] = useState<RegistrationMode>('open')
  const [registrationEnabled, setRegistrationEnabled] = useState(true)
  const [expiresAt, setExpiresAt] = useState('')
  const [maxUses, setMaxUses] = useState('0')
  const [generatedCode, setGeneratedCode] = useState('')

  const enterpriseQuery = useQuery({
    queryKey: ['enterprise', enterpriseId],
    queryFn: async () => {
      if (!enterpriseId) return null
      const response = await getEnterprise(enterpriseId)
      if (!response.success || !response.data) {
        throw new Error(
          response.message || t('Unable to load enterprise settings')
        )
      }
      setMode(normalizeMode(response.data.registration_mode))
      setRegistrationEnabled(response.data.registration_enabled)
      return response.data
    },
    enabled: Boolean(enterpriseId),
  })

  const invitationsQuery = useQuery({
    queryKey: ['enterprise', enterpriseId, 'invitations'],
    queryFn: async () => {
      if (!enterpriseId) return []
      const response = await getEnterpriseInvitations(enterpriseId)
      if (!response.success) {
        throw new Error(response.message || t('Unable to load invitations'))
      }
      return response.data?.items ?? []
    },
    enabled: Boolean(enterpriseId),
  })

  const updateSettingsMutation = useMutation({
    mutationFn: async () => {
      if (!enterpriseId) throw new Error(t('No enterprise selected'))
      return updateEnterpriseRegistration(enterpriseId, {
        registration_enabled: registrationEnabled,
        registration_mode: mode,
      })
    },
    onSuccess: (response) => {
      if (!response.success) {
        toast.error(response.message || t('Failed to update join policy'))
        return
      }
      toast.success(t('Join policy updated'))
      void queryClient.invalidateQueries({
        queryKey: ['enterprise', enterpriseId],
      })
    },
    onError: (error) => toast.error(error.message),
  })

  const createInvitationMutation = useMutation({
    mutationFn: async () => {
      if (!enterpriseId) throw new Error(t('No enterprise selected'))
      const parsedMaxUses = Number.parseInt(maxUses, 10)
      if (!Number.isInteger(parsedMaxUses) || parsedMaxUses < 0) {
        throw new Error(t('Maximum uses must be zero or a positive number'))
      }
      const parsedExpiresAt = expiresAt
        ? Math.floor(new Date(expiresAt).getTime() / 1000)
        : 0
      if (expiresAt && !Number.isFinite(parsedExpiresAt)) {
        throw new Error(t('Please enter a valid expiration time'))
      }
      return createEnterpriseInvitation(enterpriseId, {
        expires_at: parsedExpiresAt,
        max_uses: parsedMaxUses,
      })
    },
    onSuccess: (response) => {
      if (!response.success || !response.data) {
        toast.error(response.message || t('Failed to create invitation'))
        return
      }
      setGeneratedCode(response.data.code)
      setExpiresAt('')
      setMaxUses('0')
      toast.success(t('Invitation created'))
      void queryClient.invalidateQueries({
        queryKey: ['enterprise', enterpriseId, 'invitations'],
      })
    },
    onError: (error) => toast.error(error.message),
  })

  const updateInvitationMutation = useMutation({
    mutationFn: async (invitation: EnterpriseInvitation) => {
      if (!enterpriseId) throw new Error(t('No enterprise selected'))
      return updateEnterpriseInvitation(
        enterpriseId,
        invitation.id,
        invitation.status === 1 ? 2 : 1
      )
    },
    onSuccess: (response) => {
      if (!response.success) {
        toast.error(response.message || t('Failed to update invitation'))
        return
      }
      void queryClient.invalidateQueries({
        queryKey: ['enterprise', enterpriseId, 'invitations'],
      })
    },
    onError: (error) => toast.error(error.message),
  })

  const invitationColumns: StaticDataTableColumn<EnterpriseInvitation>[] = [
    {
      id: 'status',
      header: t('Status'),
      cell: (invitation) => (
        <StatusBadge
          label={t(invitation.status === 1 ? 'Active' : 'Inactive')}
          variant={invitation.status === 1 ? 'success' : 'neutral'}
          copyable={false}
        />
      ),
    },
    {
      id: 'usage',
      header: t('Usage'),
      cell: (invitation) =>
        `${invitation.used_count} / ${invitation.max_uses > 0 ? invitation.max_uses : t('Unlimited')}`,
    },
    {
      id: 'expires',
      header: t('Expires'),
      cell: (invitation) => formatTimestamp(invitation.expires_at),
    },
    {
      id: 'action',
      header: t('Actions'),
      cell: (invitation) => (
        <Button
          variant='outline'
          size='sm'
          onClick={() => updateInvitationMutation.mutate(invitation)}
          disabled={updateInvitationMutation.isPending}
        >
          {t(invitation.status === 1 ? 'Disable' : 'Enable')}
        </Button>
      ),
    },
  ]

  return (
    <EnterpriseShell
      section='onboarding'
      title={t('Enterprise settings')}
      description={t(
        'Decide how people enter this enterprise and manage invitation codes.'
      )}
    >
      {enterpriseQuery.isLoading && <LoadingState message={t('Loading...')} />}
      {enterpriseQuery.isError && (
        <ErrorState
          title={t('Unable to load enterprise settings')}
          onRetry={() => void enterpriseQuery.refetch()}
        />
      )}
      {!enterpriseQuery.isLoading && !enterpriseQuery.isError && (
        <div className='space-y-5'>
          <TitledCard
            title={t('Registration policy')}
            description={t(
              'This policy controls new registrations only; existing members are not removed.'
            )}
            icon={<Settings2 className='size-4' aria-hidden='true' />}
          >
            <div className='grid gap-4 md:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_auto] md:items-end'>
              <label className='grid gap-2 text-sm font-medium'>
                {t('Registration mode')}
                <Select
                  items={[
                    { value: 'open', label: t('Open registration') },
                    { value: 'invite', label: t('Invitation required') },
                    { value: 'closed', label: t('Closed') },
                  ]}
                  value={mode}
                  onValueChange={(value) => setMode(value as RegistrationMode)}
                >
                  <SelectTrigger
                    aria-label={t('Registration mode')}
                    className='w-full'
                  >
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent alignItemWithTrigger={false}>
                    <SelectItem value='open'>
                      {t('Open registration')}
                    </SelectItem>
                    <SelectItem value='invite'>
                      {t('Invitation required')}
                    </SelectItem>
                    <SelectItem value='closed'>{t('Closed')}</SelectItem>
                  </SelectContent>
                </Select>
              </label>
              <label className='grid gap-2 text-sm font-medium'>
                {t('Registration status')}
                <Select
                  items={[
                    { value: 'enabled', label: t('Enabled') },
                    { value: 'disabled', label: t('Disabled') },
                  ]}
                  value={registrationEnabled ? 'enabled' : 'disabled'}
                  onValueChange={(value) =>
                    setRegistrationEnabled(value === 'enabled')
                  }
                >
                  <SelectTrigger
                    aria-label={t('Registration status')}
                    className='w-full'
                  >
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent alignItemWithTrigger={false}>
                    <SelectItem value='enabled'>{t('Enabled')}</SelectItem>
                    <SelectItem value='disabled'>{t('Disabled')}</SelectItem>
                  </SelectContent>
                </Select>
              </label>
              <Button
                onClick={() => updateSettingsMutation.mutate()}
                disabled={
                  updateSettingsMutation.isPending || !enterpriseQuery.data
                }
              >
                {t('Save policy')}
              </Button>
            </div>
            <p className='text-muted-foreground mt-4 text-sm leading-6'>
              {t(policyDescription(mode))}
            </p>
          </TitledCard>

          <TitledCard
            title={t('Invitation codes')}
            description={t(
              'Use invitations when you need controlled, auditable onboarding.'
            )}
            icon={<KeyRound className='size-4' aria-hidden='true' />}
          >
            <div className='grid gap-4 md:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_auto] md:items-end'>
              <label className='grid gap-2 text-sm font-medium'>
                {t('Expires at')}
                <Input
                  type='datetime-local'
                  value={expiresAt}
                  onChange={(event) => setExpiresAt(event.target.value)}
                />
              </label>
              <label className='grid gap-2 text-sm font-medium'>
                {t('Maximum uses')}
                <Input
                  type='number'
                  min='0'
                  value={maxUses}
                  onChange={(event) => setMaxUses(event.target.value)}
                />
              </label>
              <Button
                onClick={() => createInvitationMutation.mutate()}
                disabled={
                  createInvitationMutation.isPending ||
                  mode === 'closed' ||
                  !registrationEnabled
                }
              >
                {t('Create invitation')}
              </Button>
            </div>
            {generatedCode && (
              <div className='bg-muted/50 mt-4 flex flex-wrap items-center justify-between gap-3 rounded-lg border p-3'>
                <div>
                  <p className='text-sm font-medium'>
                    {t('New invitation code')}
                  </p>
                  <code className='text-primary mt-1 block text-sm'>
                    {generatedCode}
                  </code>
                </div>
                <Button
                  variant='outline'
                  size='sm'
                  onClick={() => {
                    void navigator.clipboard.writeText(generatedCode)
                    toast.success(t('Copied'))
                  }}
                >
                  <Copy aria-hidden='true' />
                  {t('Copy')}
                </Button>
              </div>
            )}
            {invitationsQuery.isLoading && (
              <LoadingState message={t('Loading...')} />
            )}
            {invitationsQuery.isError && (
              <ErrorState
                title={t('Unable to load invitations')}
                onRetry={() => void invitationsQuery.refetch()}
              />
            )}
            {!invitationsQuery.isLoading && !invitationsQuery.isError && (
              <StaticDataTable
                columns={invitationColumns}
                data={invitationsQuery.data ?? []}
                getRowKey={(invitation) => invitation.id}
                tableClassName='mt-5 min-w-[620px]'
                emptyContent={
                  <EmptyState
                    className='min-h-[180px]'
                    title={t('No Data')}
                    description={t('No invitation codes yet.')}
                  />
                }
              />
            )}
          </TitledCard>
        </div>
      )}
    </EnterpriseShell>
  )
}

function normalizeMode(mode: string): RegistrationMode {
  if (mode === 'invite' || mode === 'closed') return mode
  return 'open'
}

function policyDescription(mode: RegistrationMode) {
  if (mode === 'invite') {
    return 'Only users with a valid invitation code can join this enterprise.'
  }
  if (mode === 'closed') {
    return 'New registration is closed and this enterprise is hidden from sign-up.'
  }
  return 'Users can choose this enterprise during registration without an invitation code.'
}

function formatTimestamp(value: number) {
  if (!value) return '--'
  return new Intl.DateTimeFormat(undefined, {
    dateStyle: 'medium',
    timeStyle: 'short',
  }).format(new Date(value * 1000))
}
