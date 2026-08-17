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
import { Link } from '@tanstack/react-router'
import { Building2, ChartNoAxesCombined, Plus } from 'lucide-react'
import { useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/components/confirm-dialog'
import {
  StaticDataTable,
  type StaticDataTableColumn,
} from '@/components/data-table'
import { EmptyState } from '@/components/empty-state'
import { ErrorState } from '@/components/error-state'
import { SectionPageLayout } from '@/components/layout'
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
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { TitledCard } from '@/components/ui/titled-card'
import { handleServerError } from '@/lib/handle-server-error'

import {
  assignEnterpriseMember,
  createEnterprise,
  getEnterpriseMemberCandidates,
  getEnterprises,
  updateEnterprise,
} from '../api'
import type { Enterprise, EnterpriseMemberCandidate } from '../types'

export function PlatformEnterprisePage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [name, setName] = useState('')
  const [code, setCode] = useState('')
  const [pendingEnterprise, setPendingEnterprise] = useState<Enterprise | null>(
    null
  )
  const [pendingOwnerEnterprise, setPendingOwnerEnterprise] =
    useState<Enterprise | null>(null)
  const [ownerKeyword, setOwnerKeyword] = useState('')
  const [ownerCandidateId, setOwnerCandidateId] = useState('')
  const query = useQuery({
    queryKey: ['enterprise-admin', 'enterprises'],
    queryFn: async () => {
      const response = await getEnterprises()
      if (!response.success) {
        throw new Error(response.message || t('Failed to load enterprises'))
      }
      return response.data?.items ?? []
    },
  })
  const createMutation = useMutation({
    mutationFn: () =>
      createEnterprise({ name: name.trim(), code: code.trim() }),
    onSuccess: async (response) => {
      if (!response.success) {
        throw new Error(response.message || t('Failed to create enterprise'))
      }
      setName('')
      setCode('')
      await queryClient.invalidateQueries({
        queryKey: ['enterprise-admin', 'enterprises'],
      })
      toast.success(t('Enterprise created'))
    },
    onError: (error) => reportError(error, t('Failed to create enterprise')),
  })
  const statusMutation = useMutation({
    mutationFn: async (enterprise: Enterprise) => {
      const response = await updateEnterprise(enterprise.id, {
        status: enterprise.status === 1 ? 2 : 1,
      })
      if (!response.success) {
        throw new Error(response.message || t('Failed to update enterprise'))
      }
    },
    onSuccess: async () => {
      setPendingEnterprise(null)
      await queryClient.invalidateQueries({
        queryKey: ['enterprise-admin', 'enterprises'],
      })
      toast.success(t('Enterprise updated'))
    },
    onError: (error) => reportError(error, t('Failed to update enterprise')),
  })
  const ownerCandidatesQuery = useQuery({
    queryKey: [
      'enterprise-admin',
      'owner-candidates',
      pendingOwnerEnterprise?.id,
      ownerKeyword,
    ],
    queryFn: async () => {
      if (!pendingOwnerEnterprise) return []
      const response = await getEnterpriseMemberCandidates(
        pendingOwnerEnterprise.id,
        ownerKeyword
      )
      if (!response.success) {
        throw new Error(
          response.message || t('Unable to load member candidates')
        )
      }
      return response.data ?? []
    },
    enabled: Boolean(pendingOwnerEnterprise),
  })
  const ownerMutation = useMutation({
    mutationFn: async () => {
      if (!pendingOwnerEnterprise || !ownerCandidateId) {
        throw new Error(t('Please select a user'))
      }
      return assignEnterpriseMember(pendingOwnerEnterprise.id, {
        user_id: Number(ownerCandidateId),
        role: 'owner',
      })
    },
    onSuccess: async (response) => {
      if (!response.success) {
        toast.error(response.message || t('Failed to assign owner'))
        return
      }
      setPendingOwnerEnterprise(null)
      setOwnerCandidateId('')
      setOwnerKeyword('')
      toast.success(t('Owner assigned'))
    },
    onError: (error) => reportError(error, t('Failed to assign owner')),
  })
  const columns: StaticDataTableColumn<Enterprise>[] = [
    {
      id: 'enterprise',
      header: t('Enterprise'),
      cell: (row) => (
        <div className='flex min-w-0 items-center gap-3'>
          <div className='bg-primary/10 text-primary flex size-8 shrink-0 items-center justify-center rounded-lg'>
            <Building2 className='size-4' aria-hidden='true' />
          </div>
          <div className='min-w-0'>
            <p className='truncate font-medium'>{row.name}</p>
            <p className='text-muted-foreground mt-0.5 truncate font-mono text-xs'>
              {row.code}
            </p>
          </div>
        </div>
      ),
    },
    {
      id: 'registration',
      header: t('Registration'),
      cell: (row) => (row.registration_enabled ? t('Enabled') : t('Disabled')),
    },
    {
      id: 'status',
      header: t('Status'),
      cell: (row) => (
        <StatusBadge
          label={t(row.status === 1 ? 'Active' : 'Inactive')}
          variant={row.status === 1 ? 'success' : 'neutral'}
          copyable={false}
        />
      ),
    },
    {
      id: 'actions',
      header: '',
      className: 'text-right',
      cellClassName: 'text-right',
      cell: (row) => (
        <div className='flex justify-end gap-2'>
          <Button
            type='button'
            size='sm'
            variant='outline'
            aria-haspopup='dialog'
            aria-expanded={pendingOwnerEnterprise?.id === row.id}
            onClick={() => setPendingOwnerEnterprise(row)}
          >
            {t('Set owner')}
          </Button>
          <Button
            type='button'
            size='sm'
            variant='outline'
            onClick={() => setPendingEnterprise(row)}
          >
            {t(row.status === 1 ? 'Disable' : 'Enable')}
          </Button>
        </div>
      ),
    },
  ]

  function handleCreate(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!name.trim() || !code.trim()) return
    createMutation.mutate()
  }

  return (
    <>
      <SectionPageLayout>
        <SectionPageLayout.Title>
          {t('Enterprise management')}
        </SectionPageLayout.Title>
        <SectionPageLayout.Content>
          <div className='mx-auto w-full max-w-[1440px] space-y-5'>
            <div className='border-border/60 bg-card rounded-xl border p-4 shadow-xs sm:p-5'>
              <div className='flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between'>
                <div>
                  <p className='text-muted-foreground text-xs font-medium tracking-wide uppercase'>
                    {t('Platform')}
                  </p>
                  <h1 className='mt-1 text-xl font-semibold tracking-tight'>
                    {t('Enterprise management')}
                  </h1>
                  <p className='text-muted-foreground mt-1 max-w-2xl text-sm'>
                    {t('Create and maintain customer enterprise tenants.')}
                  </p>
                </div>
                <div className='flex flex-wrap items-center gap-2'>
                  <Button
                    variant='outline'
                    render={<Link to='/enterprise/admin/rankings' />}
                    nativeButton={false}
                  >
                    <ChartNoAxesCombined aria-hidden='true' />
                    {t('Enterprise consumption rankings')}
                  </Button>
                  <StatusBadge
                    label={t('Super Admin')}
                    variant='info'
                    copyable={false}
                  />
                </div>
              </div>
            </div>

            <TitledCard
              title={t('Create enterprise')}
              description={t(
                'Create the tenant first, then appoint its owner and configure its join policy.'
              )}
              icon={<Plus className='size-4' aria-hidden='true' />}
            >
              <form
                className='grid gap-3 md:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_auto]'
                onSubmit={handleCreate}
              >
                <label className='grid gap-1.5 text-sm'>
                  <span>{t('Enterprise name')}</span>
                  <Input
                    value={name}
                    onChange={(event) => setName(event.target.value)}
                    required
                  />
                </label>
                <label className='grid gap-1.5 text-sm'>
                  <span>{t('Enterprise code')}</span>
                  <Input
                    value={code}
                    onChange={(event) => setCode(event.target.value)}
                    required
                  />
                </label>
                <Button
                  type='submit'
                  className='md:self-end'
                  disabled={createMutation.isPending}
                >
                  {t('Create enterprise')}
                </Button>
              </form>
            </TitledCard>

            <TitledCard
              title={t('Enterprises')}
              description={t(
                'Platform administrators can manage status here; daily member operations belong to the enterprise workspace.'
              )}
              icon={<Building2 className='size-4' aria-hidden='true' />}
            >
              {query.isLoading && <LoadingState message={t('Loading...')} />}
              {query.isError && (
                <ErrorState
                  title={t('Failed to load enterprises')}
                  onRetry={() => void query.refetch()}
                />
              )}
              {!query.isLoading && !query.isError && (
                <StaticDataTable
                  columns={columns}
                  data={query.data ?? []}
                  getRowKey={(row) => row.id}
                  tableClassName='min-w-[680px]'
                  emptyContent={
                    <EmptyState
                      className='min-h-[220px]'
                      title={t('No Data')}
                      description={t('No enterprises yet.')}
                    />
                  }
                />
              )}
            </TitledCard>
          </div>
        </SectionPageLayout.Content>
      </SectionPageLayout>
      <ConfirmDialog
        open={pendingEnterprise != null}
        onOpenChange={(open) => {
          if (!open) setPendingEnterprise(null)
        }}
        title={t(
          pendingEnterprise?.status === 1
            ? 'Disable enterprise'
            : 'Enable enterprise'
        )}
        desc={t(
          'This changes whether the enterprise and its members can continue using the service.'
        )}
        destructive={pendingEnterprise?.status === 1}
        confirmText={t('Continue')}
        handleConfirm={() => {
          if (pendingEnterprise) statusMutation.mutate(pendingEnterprise)
        }}
        isLoading={statusMutation.isPending}
      />
      <Sheet
        open={pendingOwnerEnterprise != null}
        onOpenChange={(open) => {
          if (!open) {
            setPendingOwnerEnterprise(null)
            setOwnerCandidateId('')
            setOwnerKeyword('')
          }
        }}
      >
        <SheetContent className='sm:max-w-lg'>
          <SheetHeader>
            <SheetTitle>{t('Assign enterprise owner')}</SheetTitle>
            <SheetDescription>
              {t('The owner manages the enterprise workspace and its members.')}
            </SheetDescription>
          </SheetHeader>
          <div className='min-h-0 flex-1 space-y-4 overflow-y-auto p-4'>
            <Input
              value={ownerKeyword}
              onChange={(event) => setOwnerKeyword(event.target.value)}
              placeholder={t('Search by username, email, or display name')}
              aria-label={t('Search users')}
            />
            <Select
              items={
                ownerCandidatesQuery.data?.map((candidate) => ({
                  value: String(candidate.id),
                  label: `${candidate.display_name || candidate.username} · ${candidate.email || candidate.username}`,
                })) ?? []
              }
              value={ownerCandidateId}
              onValueChange={(value) => setOwnerCandidateId(value ?? '')}
            >
              <SelectTrigger aria-label={t('Select user')} className='w-full'>
                <SelectValue
                  placeholder={
                    ownerCandidatesQuery.isLoading
                      ? t('Loading...')
                      : t('Select a user')
                  }
                />
              </SelectTrigger>
              <SelectContent alignItemWithTrigger={false}>
                {ownerCandidatesQuery.data?.map(
                  (candidate: EnterpriseMemberCandidate) => (
                    <SelectItem key={candidate.id} value={String(candidate.id)}>
                      {candidate.display_name || candidate.username} ·{' '}
                      {candidate.email || candidate.username}
                    </SelectItem>
                  )
                )}
              </SelectContent>
            </Select>
            {ownerCandidatesQuery.isError && (
              <p className='text-destructive text-sm'>
                {t('Unable to load member candidates')}
              </p>
            )}
          </div>
          <SheetFooter>
            <Button
              onClick={() => ownerMutation.mutate()}
              disabled={!ownerCandidateId || ownerMutation.isPending}
            >
              {t('Assign owner')}
            </Button>
          </SheetFooter>
        </SheetContent>
      </Sheet>
    </>
  )
}

function reportError(error: unknown, fallback: string) {
  if (error instanceof Error) {
    toast.error(error.message || fallback)
    return
  }
  handleServerError(error)
}
