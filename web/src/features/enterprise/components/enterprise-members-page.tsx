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
import { Plus, Users } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/components/confirm-dialog'
import {
  StaticDataTable,
  type StaticDataTableColumn,
} from '@/components/data-table'
import { EmptyState } from '@/components/empty-state'
import { ErrorState } from '@/components/error-state'
import { LoadingState } from '@/components/loading-state'
import { StatusBadge } from '@/components/status-badge'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
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
  assignEnterpriseMember,
  getEnterpriseMemberCandidates,
  getEnterpriseMembers,
  updateEnterpriseMember,
} from '../api'
import type { EnterpriseMemberCandidate, EnterpriseMembership } from '../types'
import { EnterpriseShell } from './enterprise-shell'

type PendingMemberUpdate = {
  member: EnterpriseMembership
  role: string
  status: number
}

export function EnterpriseMembersPage() {
  const { t } = useTranslation()
  const user = useAuthStore((state) => state.auth.user)
  const enterpriseId = user?.enterprise?.id
  const queryClient = useQueryClient()
  const [pendingUpdate, setPendingUpdate] =
    useState<PendingMemberUpdate | null>(null)
  const [isAddDialogOpen, setIsAddDialogOpen] = useState(false)
  const [candidateKeyword, setCandidateKeyword] = useState('')
  const [candidateId, setCandidateId] = useState('')
  const [candidateRole, setCandidateRole] = useState('member')
  const query = useQuery({
    queryKey: ['enterprise', enterpriseId, 'members'],
    queryFn: async () => {
      if (!enterpriseId) return []
      const response = await getEnterpriseMembers(enterpriseId)
      if (!response.success) {
        throw new Error(
          response.message || t('Unable to load enterprise management data')
        )
      }
      return response.data?.items ?? []
    },
    enabled: Boolean(enterpriseId),
  })

  const candidatesQuery = useQuery({
    queryKey: [
      'enterprise',
      enterpriseId,
      'member-candidates',
      candidateKeyword,
    ],
    queryFn: async () => {
      if (!enterpriseId) return []
      const response = await getEnterpriseMemberCandidates(
        enterpriseId,
        candidateKeyword
      )
      if (!response.success) {
        throw new Error(
          response.message || t('Unable to load member candidates')
        )
      }
      return response.data ?? []
    },
    enabled: Boolean(enterpriseId && isAddDialogOpen),
  })

  const updateMutation = useMutation({
    mutationFn: async (update: PendingMemberUpdate) => {
      if (!enterpriseId) throw new Error(t('No enterprise selected'))
      return updateEnterpriseMember(enterpriseId, update.member.user_id, {
        role: update.role,
        status: update.status,
      })
    },
    onSuccess: (response) => {
      if (!response.success) {
        toast.error(response.message || t('Failed to update member'))
        return
      }
      toast.success(t('Member updated'))
      setPendingUpdate(null)
      void queryClient.invalidateQueries({
        queryKey: ['enterprise', enterpriseId, 'members'],
      })
    },
    onError: (error) => toast.error(error.message),
  })

  const addMutation = useMutation({
    mutationFn: async () => {
      if (!enterpriseId || !candidateId) {
        throw new Error(t('Please select a user'))
      }
      return assignEnterpriseMember(enterpriseId, {
        user_id: Number(candidateId),
        role: candidateRole,
      })
    },
    onSuccess: (response) => {
      if (!response.success) {
        toast.error(response.message || t('Failed to add member'))
        return
      }
      toast.success(t('Member added'))
      setIsAddDialogOpen(false)
      setCandidateId('')
      setCandidateKeyword('')
      void queryClient.invalidateQueries({
        queryKey: ['enterprise', enterpriseId, 'members'],
      })
    },
    onError: (error) => toast.error(error.message),
  })

  const columns: StaticDataTableColumn<EnterpriseMembership>[] = [
    {
      id: 'member',
      header: t('Member'),
      cell: (row) => (
        <div>
          <p className='font-medium'>
            {row.user?.display_name ||
              row.user?.username ||
              `${t('User')} #${row.user_id}`}
          </p>
          <p className='text-muted-foreground mt-0.5 font-mono text-xs'>
            {row.user?.email || `ID ${row.user_id}`}
          </p>
        </div>
      ),
    },
    {
      id: 'role',
      header: t('Role'),
      cell: (row) => (
        <Select
          items={[
            ...(row.role === 'owner'
              ? [{ value: 'owner', label: t('Owner') }]
              : []),
            { value: 'member', label: t('Member') },
            { value: 'admin', label: t('Admin') },
            { value: 'auditor', label: t('Auditor') },
          ]}
          value={row.role}
          onValueChange={(value) =>
            setPendingUpdate({
              member: row,
              role: value ?? row.role,
              status: row.status,
            })
          }
        >
          <SelectTrigger size='sm' aria-label={t('Role')} className='w-[120px]'>
            <SelectValue />
          </SelectTrigger>
          <SelectContent alignItemWithTrigger={false}>
            {row.role === 'owner' && (
              <SelectItem value='owner' disabled>
                {t('Owner')}
              </SelectItem>
            )}
            <SelectItem value='member'>{t('Member')}</SelectItem>
            <SelectItem value='admin'>{t('Admin')}</SelectItem>
            <SelectItem value='auditor'>{t('Auditor')}</SelectItem>
          </SelectContent>
        </Select>
      ),
    },
    {
      id: 'status',
      header: t('Status'),
      cell: (row) => (
        <div className='flex items-center gap-2'>
          <StatusBadge
            label={t(row.status === 1 ? 'Active' : 'Inactive')}
            variant={row.status === 1 ? 'success' : 'neutral'}
            copyable={false}
          />
          <Button
            variant='outline'
            size='sm'
            onClick={() =>
              setPendingUpdate({
                member: row,
                role: row.role,
                status: row.status === 1 ? 2 : 1,
              })
            }
          >
            {t(row.status === 1 ? 'Disable' : 'Enable')}
          </Button>
        </div>
      ),
    },
    {
      id: 'joined-at',
      header: t('Joined at'),
      cell: (row) => formatTimestamp(row.joined_at),
    },
  ]

  return (
    <EnterpriseShell
      section='members'
      title={t('Enterprise members')}
      description={t('Manage people and roles in your enterprise.')}
    >
      <TitledCard
        title={t('Enterprise members')}
        description={t(
          'Members are linked to the enterprise through membership records.'
        )}
        icon={<Users className='size-4' aria-hidden='true' />}
        action={
          <Button onClick={() => setIsAddDialogOpen(true)}>
            <Plus aria-hidden='true' />
            {t('Add member')}
          </Button>
        }
      >
        {query.isLoading && <LoadingState message={t('Loading...')} />}
        {query.isError && (
          <ErrorState
            title={t('Unable to load enterprise management data')}
            onRetry={() => void query.refetch()}
          />
        )}
        {!query.isLoading && !query.isError && (
          <StaticDataTable
            columns={columns}
            data={query.data ?? []}
            getRowKey={(row) => row.id}
            tableClassName='min-w-[620px]'
            emptyContent={
              <EmptyState
                className='min-h-[220px]'
                title={t('No Data')}
                description={t('No enterprise members yet.')}
              />
            }
          />
        )}
      </TitledCard>
      <ConfirmDialog
        open={Boolean(pendingUpdate)}
        onOpenChange={(open) => !open && setPendingUpdate(null)}
        title={t('Confirm member update')}
        desc={t('This changes the member role or access status immediately.')}
        confirmText={t('Save changes')}
        handleConfirm={() => {
          if (pendingUpdate) updateMutation.mutate(pendingUpdate)
        }}
        isLoading={updateMutation.isPending}
      />
      <Dialog
        open={isAddDialogOpen}
        onOpenChange={(open) => {
          setIsAddDialogOpen(open)
          if (!open) {
            setCandidateId('')
            setCandidateKeyword('')
          }
        }}
      >
        <DialogContent className='sm:max-w-lg'>
          <DialogHeader>
            <DialogTitle>{t('Add member')}</DialogTitle>
            <DialogDescription>
              {t('Find an existing user and assign them to this enterprise.')}
            </DialogDescription>
          </DialogHeader>
          <div className='space-y-4'>
            <Input
              value={candidateKeyword}
              onChange={(event) => setCandidateKeyword(event.target.value)}
              placeholder={t('Search by username, email, or display name')}
              aria-label={t('Search users')}
            />
            <Select
              items={
                candidatesQuery.data?.map((candidate) => ({
                  value: String(candidate.id),
                  label: `${candidate.display_name || candidate.username} · ${candidate.email || candidate.username}`,
                })) ?? []
              }
              value={candidateId}
              onValueChange={(value) => setCandidateId(value ?? '')}
            >
              <SelectTrigger aria-label={t('Select user')} className='w-full'>
                <SelectValue
                  placeholder={
                    candidatesQuery.isLoading
                      ? t('Loading...')
                      : t('Select a user')
                  }
                />
              </SelectTrigger>
              <SelectContent alignItemWithTrigger={false}>
                {candidatesQuery.data?.map(
                  (candidate: EnterpriseMemberCandidate) => (
                    <SelectItem key={candidate.id} value={String(candidate.id)}>
                      {candidate.display_name || candidate.username} ·{' '}
                      {candidate.email || candidate.username}
                    </SelectItem>
                  )
                )}
              </SelectContent>
            </Select>
            {candidatesQuery.isError && (
              <p className='text-destructive text-sm'>
                {t('Unable to load member candidates')}
              </p>
            )}
            <Select
              items={[
                { value: 'member', label: t('Member') },
                { value: 'admin', label: t('Admin') },
                { value: 'auditor', label: t('Auditor') },
              ]}
              value={candidateRole}
              onValueChange={(value) => setCandidateRole(value ?? 'member')}
            >
              <SelectTrigger aria-label={t('Role')} className='w-full'>
                <SelectValue />
              </SelectTrigger>
              <SelectContent alignItemWithTrigger={false}>
                <SelectItem value='member'>{t('Member')}</SelectItem>
                <SelectItem value='admin'>{t('Admin')}</SelectItem>
                <SelectItem value='auditor'>{t('Auditor')}</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <DialogFooter>
            <Button
              onClick={() => addMutation.mutate()}
              disabled={!candidateId || addMutation.isPending}
            >
              {t('Add member')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </EnterpriseShell>
  )
}

function formatTimestamp(value: number) {
  if (!value) return '--'
  return new Intl.DateTimeFormat(undefined, {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
  }).format(new Date(value * 1000))
}
