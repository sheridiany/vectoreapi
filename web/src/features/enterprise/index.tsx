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
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useEffect, useMemo, useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { SectionPageLayout } from '@/components/layout'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { handleServerError } from '@/lib/handle-server-error'
import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'

import {
  assignEnterpriseMember,
  createEnterprise,
  createEnterpriseInvitation,
  getEnterpriseInvitations,
  getEnterpriseMembers,
  getEnterprises,
  updateEnterprise,
  updateEnterpriseInvitation,
  updateEnterpriseMember,
} from './api'
import { useEnterpriseRankings } from './hooks/use-enterprise-rankings'
import {
  canAppointEnterpriseOwner,
  canManageEnterprise,
} from './lib/permissions'
import type {
  Enterprise,
  EnterpriseInvitation,
  EnterpriseMembership,
  EnterpriseRanking,
  EnterpriseRankingPeriod,
} from './types'

const periods: { id: EnterpriseRankingPeriod; label: string }[] = [
  { id: 'today', label: 'Today' },
  { id: 'week', label: 'This week' },
  { id: 'month', label: 'This month' },
  { id: 'custom', label: 'Custom range' },
]

function formatNumber(value: number) {
  return new Intl.NumberFormat().format(value)
}

function formatGrowth(value: number) {
  return `${value >= 0 ? '+' : ''}${value.toFixed(1)}%`
}

function reportEnterpriseError(error: unknown, fallback: string) {
  if (error instanceof Error) {
    toast.error(fallback)
    return
  }
  handleServerError(error)
}

export function Enterprise() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const user = useAuthStore((state) => state.auth.user)
  const [period, setPeriod] = useState<EnterpriseRankingPeriod>('week')
  const [customStart, setCustomStart] = useState('')
  const [customEnd, setCustomEnd] = useState('')
  const isRoot = user?.role === ROLE.SUPER_ADMIN
  const canManage = canManageEnterprise(user)
  const canAppointOwner = canAppointEnterpriseOwner(user)
  const enterpriseId = isRoot ? undefined : user?.enterprise?.id
  const [selectedEnterpriseId, setSelectedEnterpriseId] = useState<number>()
  const [enterpriseName, setEnterpriseName] = useState('')
  const [enterpriseCode, setEnterpriseCode] = useState('')
  const [memberUserId, setMemberUserId] = useState('')
  const [memberRole, setMemberRole] = useState('member')
  const [invitationMaxUses, setInvitationMaxUses] = useState('0')
  const managedEnterpriseId = isRoot ? selectedEnterpriseId : enterpriseId
  const customRange = useMemo(() => {
    if (period !== 'custom' || !customStart || !customEnd) return undefined
    return {
      start: Math.floor(new Date(`${customStart}T00:00:00`).getTime() / 1000),
      end: Math.floor(new Date(`${customEnd}T23:59:59`).getTime() / 1000),
    }
  }, [customEnd, customStart, period])
  const rankingQueryEnabled =
    (isRoot || Boolean(enterpriseId && canManage)) &&
    (period !== 'custom' || Boolean(customRange))
  const rankingsQuery = useEnterpriseRankings(
    enterpriseId,
    period,
    customRange?.start,
    customRange?.end,
    rankingQueryEnabled
  )
  const memberRankingsQuery = useEnterpriseRankings(
    managedEnterpriseId,
    period,
    customRange?.start,
    customRange?.end,
    Boolean(managedEnterpriseId && canManage) &&
      (period !== 'custom' || Boolean(customRange))
  )
  const data = rankingsQuery.data?.data
  const memberRankingData = memberRankingsQuery.data?.data

  const enterprisesQuery = useQuery({
    queryKey: ['enterprise-admin', 'enterprises'],
    queryFn: async () => {
      const response = await getEnterprises()
      if (!response.success) {
        throw new Error(response.message || t('Failed to load enterprises'))
      }
      return response.data?.items ?? []
    },
    enabled: isRoot,
  })
  const enterprises = useMemo(
    () => enterprisesQuery.data ?? [],
    [enterprisesQuery.data]
  )

  useEffect(() => {
    if (!isRoot) return
    setSelectedEnterpriseId((current) => current ?? enterprises[0]?.id)
  }, [enterprises, isRoot])

  const membersQuery = useQuery({
    queryKey: ['enterprise', managedEnterpriseId, 'members'],
    queryFn: async () => {
      if (!managedEnterpriseId) return []
      const response = await getEnterpriseMembers(managedEnterpriseId)
      if (!response.success) {
        throw new Error(
          response.message || t('Unable to load enterprise management data')
        )
      }
      return response.data?.items ?? []
    },
    enabled: Boolean(managedEnterpriseId && canManage),
  })
  const invitationsQuery = useQuery({
    queryKey: ['enterprise', managedEnterpriseId, 'invitations'],
    queryFn: async () => {
      if (!managedEnterpriseId) return []
      const response = await getEnterpriseInvitations(managedEnterpriseId)
      if (!response.success) {
        throw new Error(
          response.message || t('Unable to load enterprise management data')
        )
      }
      return response.data?.items ?? []
    },
    enabled: Boolean(managedEnterpriseId && canManage),
  })
  const members = membersQuery.data ?? []
  const invitations = invitationsQuery.data ?? []

  const createEnterpriseMutation = useMutation({
    mutationFn: async (payload: { name: string; code: string }) => {
      const response = await createEnterprise(payload)
      if (!response.success || !response.data) {
        throw new Error(response.message || t('Failed to create enterprise'))
      }
      return response.data
    },
    onSuccess: async (createdEnterprise) => {
      setEnterpriseName('')
      setEnterpriseCode('')
      setSelectedEnterpriseId(createdEnterprise.id)
      await queryClient.invalidateQueries({
        queryKey: ['enterprise-admin', 'enterprises'],
      })
      toast.success(t('Enterprise created'))
    },
    onError: (error) =>
      reportEnterpriseError(error, t('Failed to create enterprise')),
  })

  const updateEnterpriseMutation = useMutation({
    mutationFn: async (enterprise: Enterprise) => {
      const response = await updateEnterprise(enterprise.id, {
        status: enterprise.status === 1 ? 2 : 1,
      })
      if (!response.success || !response.data) {
        throw new Error(response.message || t('Failed to update enterprise'))
      }
      return response.data
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({
        queryKey: ['enterprise-admin', 'enterprises'],
      })
      toast.success(t('Enterprise updated'))
    },
    onError: (error) =>
      reportEnterpriseError(error, t('Failed to update enterprise')),
  })

  const assignMemberMutation = useMutation({
    mutationFn: async (payload: {
      enterpriseId: number
      userId: number
      role: string
    }) => {
      const response = await assignEnterpriseMember(payload.enterpriseId, {
        user_id: payload.userId,
        role: payload.role,
      })
      if (!response.success) {
        throw new Error(response.message || t('Failed to add member'))
      }
      return payload.enterpriseId
    },
    onSuccess: async (enterpriseId) => {
      setMemberUserId('')
      await queryClient.invalidateQueries({
        queryKey: ['enterprise', enterpriseId],
      })
      toast.success(t('Member updated'))
    },
    onError: (error) => reportEnterpriseError(error, t('Failed to add member')),
  })

  const updateMemberMutation = useMutation({
    mutationFn: async (payload: {
      enterpriseId: number
      membership: EnterpriseMembership
    }) => {
      const response = await updateEnterpriseMember(
        payload.enterpriseId,
        payload.membership.user_id,
        {
          role: payload.membership.role,
          status: payload.membership.status,
        }
      )
      if (!response.success) {
        throw new Error(response.message || t('Failed to update member'))
      }
      return payload.enterpriseId
    },
    onSuccess: async (enterpriseId) => {
      await queryClient.invalidateQueries({
        queryKey: ['enterprise', enterpriseId],
      })
      toast.success(t('Member updated'))
    },
    onError: (error) =>
      reportEnterpriseError(error, t('Failed to update member')),
  })

  const createInvitationMutation = useMutation({
    mutationFn: async (payload: { enterpriseId: number; maxUses: number }) => {
      const response = await createEnterpriseInvitation(payload.enterpriseId, {
        expires_at: 0,
        max_uses: payload.maxUses,
      })
      if (!response.success || !response.data) {
        throw new Error(response.message || t('Failed to create invitation'))
      }
      return { enterpriseId: payload.enterpriseId, code: response.data.code }
    },
    onSuccess: async ({ enterpriseId, code }) => {
      await queryClient.invalidateQueries({
        queryKey: ['enterprise', enterpriseId],
      })
      toast.success(`${t('Invitation created')}: ${code}`)
    },
    onError: (error) =>
      reportEnterpriseError(error, t('Failed to create invitation')),
  })

  const updateInvitationMutation = useMutation({
    mutationFn: async (payload: {
      enterpriseId: number
      invitation: EnterpriseInvitation
    }) => {
      const response = await updateEnterpriseInvitation(
        payload.enterpriseId,
        payload.invitation.id,
        payload.invitation.status === 1 ? 2 : 1
      )
      if (!response.success) {
        throw new Error(response.message || t('Failed to update invitation'))
      }
      return payload.enterpriseId
    },
    onSuccess: async (enterpriseId) => {
      await queryClient.invalidateQueries({
        queryKey: ['enterprise', enterpriseId],
      })
      toast.success(t('Invitation updated'))
    },
    onError: (error) =>
      reportEnterpriseError(error, t('Failed to update invitation')),
  })

  function handleCreateEnterprise(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!isRoot) return
    createEnterpriseMutation.mutate({
      name: enterpriseName,
      code: enterpriseCode,
    })
  }

  function handleToggleEnterprise(enterprise: Enterprise) {
    if (!isRoot) return
    updateEnterpriseMutation.mutate(enterprise)
  }

  function handleAddMember(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!managedEnterpriseId || !canManage) return
    const userId = Number(memberUserId)
    if (!Number.isInteger(userId) || userId <= 0) {
      toast.error(t('Please enter a valid user ID'))
      return
    }
    assignMemberMutation.mutate({
      enterpriseId: managedEnterpriseId,
      userId,
      role: memberRole,
    })
  }

  function handleUpdateMember(membership: EnterpriseMembership) {
    if (!managedEnterpriseId || !canManage) return
    updateMemberMutation.mutate({
      enterpriseId: managedEnterpriseId,
      membership,
    })
  }

  function handleCreateInvitation(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!managedEnterpriseId || !canManage) return
    const maxUses = Number(invitationMaxUses)
    if (!Number.isInteger(maxUses) || maxUses < 0) {
      toast.error(t('Please enter a valid maximum uses'))
      return
    }
    createInvitationMutation.mutate({
      enterpriseId: managedEnterpriseId,
      maxUses,
    })
  }

  function handleToggleInvitation(invitation: EnterpriseInvitation) {
    if (!managedEnterpriseId || !canManage) return
    updateInvitationMutation.mutate({
      enterpriseId: managedEnterpriseId,
      invitation,
    })
  }

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('Enterprise')}</SectionPageLayout.Title>
      <SectionPageLayout.Content>
        <div className='space-y-4'>
          <EnterpriseManagement
            isRoot={isRoot}
            canManage={canManage}
            canAppointOwner={canAppointOwner}
            enterprises={enterprises}
            selectedEnterpriseId={managedEnterpriseId}
            onSelectEnterprise={setSelectedEnterpriseId}
            onCreateEnterprise={handleCreateEnterprise}
            onToggleEnterprise={handleToggleEnterprise}
            enterpriseName={enterpriseName}
            enterpriseCode={enterpriseCode}
            onEnterpriseNameChange={setEnterpriseName}
            onEnterpriseCodeChange={setEnterpriseCode}
            members={members}
            invitations={invitations}
            memberUserId={memberUserId}
            memberRole={memberRole}
            onMemberUserIdChange={setMemberUserId}
            onMemberRoleChange={setMemberRole}
            onAddMember={handleAddMember}
            onUpdateMember={handleUpdateMember}
            invitationMaxUses={invitationMaxUses}
            onInvitationMaxUsesChange={setInvitationMaxUses}
            onCreateInvitation={handleCreateInvitation}
            onToggleInvitation={handleToggleInvitation}
          />
          <div className='flex flex-wrap items-center gap-2'>
            {periods.map((item) => (
              <button
                key={item.id}
                type='button'
                aria-pressed={period === item.id}
                onClick={() => setPeriod(item.id)}
                className={`rounded-md border px-3 py-2 text-sm ${period === item.id ? 'bg-primary text-primary-foreground' : 'bg-background'}`}
              >
                {t(item.label)}
              </button>
            ))}
            {period === 'custom' && (
              <>
                <input
                  aria-label={t('Start date')}
                  type='date'
                  value={customStart}
                  onChange={(event) => setCustomStart(event.target.value)}
                  className='border-input bg-background rounded-md border px-2 py-2 text-sm'
                />
                <input
                  aria-label={t('End date')}
                  type='date'
                  value={customEnd}
                  onChange={(event) => setCustomEnd(event.target.value)}
                  className='border-input bg-background rounded-md border px-2 py-2 text-sm'
                />
              </>
            )}
          </div>

          {rankingsQuery.isLoading && (
            <p className='text-muted-foreground'>{t('Loading rankings')}</p>
          )}
          {rankingsQuery.isError && (
            <p className='text-destructive'>
              {t('Unable to load enterprise rankings')}
            </p>
          )}
          {!rankingsQuery.isLoading && !rankingsQuery.isError && data && (
            <>
              {isRoot ? (
                <EnterpriseRankingTable data={data.enterprises} />
              ) : (
                <>
                  {data.enterprise && (
                    <EnterpriseRankingTable data={[data.enterprise]} />
                  )}
                  <MemberRankingTable data={data.members ?? []} />
                </>
              )}
              {isRoot && managedEnterpriseId && memberRankingData && (
                <MemberRankingTable data={memberRankingData.members ?? []} />
              )}
            </>
          )}
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}

interface EnterpriseManagementProps {
  isRoot: boolean
  canManage: boolean
  canAppointOwner: boolean
  enterprises: Enterprise[]
  selectedEnterpriseId?: number
  onSelectEnterprise: (id: number) => void
  onCreateEnterprise: (event: FormEvent<HTMLFormElement>) => void
  onToggleEnterprise: (enterprise: Enterprise) => void
  enterpriseName: string
  enterpriseCode: string
  onEnterpriseNameChange: (value: string) => void
  onEnterpriseCodeChange: (value: string) => void
  members: EnterpriseMembership[]
  invitations: EnterpriseInvitation[]
  memberUserId: string
  memberRole: string
  onMemberUserIdChange: (value: string) => void
  onMemberRoleChange: (value: string) => void
  onAddMember: (event: FormEvent<HTMLFormElement>) => void
  onUpdateMember: (membership: EnterpriseMembership) => void
  invitationMaxUses: string
  onInvitationMaxUsesChange: (value: string) => void
  onCreateInvitation: (event: FormEvent<HTMLFormElement>) => void
  onToggleInvitation: (invitation: EnterpriseInvitation) => void
}

function EnterpriseManagement(props: EnterpriseManagementProps) {
  const { t } = useTranslation()

  return (
    <div className='space-y-4'>
      {props.isRoot && (
        <Card>
          <CardHeader>
            <CardTitle>{t('Enterprise management')}</CardTitle>
          </CardHeader>
          <CardContent className='space-y-4'>
            <form
              className='flex flex-wrap items-end gap-2'
              onSubmit={props.onCreateEnterprise}
            >
              <label className='grid gap-1 text-sm'>
                {t('Enterprise name')}
                <Input
                  required
                  value={props.enterpriseName}
                  onChange={(event) =>
                    props.onEnterpriseNameChange(event.target.value)
                  }
                />
              </label>
              <label className='grid gap-1 text-sm'>
                {t('Enterprise code')}
                <Input
                  required
                  value={props.enterpriseCode}
                  onChange={(event) =>
                    props.onEnterpriseCodeChange(event.target.value)
                  }
                />
              </label>
              <Button type='submit'>{t('Create enterprise')}</Button>
            </form>
            <div className='overflow-x-auto'>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t('Enterprise')}</TableHead>
                    <TableHead>{t('Code')}</TableHead>
                    <TableHead>{t('Status')}</TableHead>
                    <TableHead />
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {props.enterprises.map((enterprise) => (
                    <TableRow key={enterprise.id}>
                      <TableCell>{enterprise.name}</TableCell>
                      <TableCell>{enterprise.code}</TableCell>
                      <TableCell>
                        {enterprise.status === 1 ? t('Enabled') : t('Disabled')}
                      </TableCell>
                      <TableCell>
                        <Button
                          size='sm'
                          variant='outline'
                          type='button'
                          onClick={() => props.onToggleEnterprise(enterprise)}
                        >
                          {enterprise.status === 1 ? t('Disable') : t('Enable')}
                        </Button>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          </CardContent>
        </Card>
      )}

      {props.selectedEnterpriseId && props.canManage && (
        <>
          <Card>
            <CardHeader>
              <CardTitle>{t('Enterprise members')}</CardTitle>
            </CardHeader>
            <CardContent className='space-y-4'>
              {props.isRoot && (
                <label className='grid max-w-sm gap-1 text-sm'>
                  {t('Select an enterprise to manage')}
                  <NativeSelect
                    value={props.selectedEnterpriseId}
                    onChange={(event) =>
                      props.onSelectEnterprise(Number(event.target.value))
                    }
                  >
                    {props.enterprises.map((enterprise) => (
                      <NativeSelectOption
                        key={enterprise.id}
                        value={enterprise.id}
                      >
                        {enterprise.name}
                      </NativeSelectOption>
                    ))}
                  </NativeSelect>
                </label>
              )}
              <form
                className='flex flex-wrap items-end gap-2'
                onSubmit={props.onAddMember}
              >
                <label className='grid gap-1 text-sm'>
                  {t('User ID')}
                  <Input
                    required
                    inputMode='numeric'
                    value={props.memberUserId}
                    onChange={(event) =>
                      props.onMemberUserIdChange(event.target.value)
                    }
                  />
                </label>
                <label className='grid gap-1 text-sm'>
                  {t('Role')}
                  <NativeSelect
                    value={props.memberRole}
                    onChange={(event) =>
                      props.onMemberRoleChange(event.target.value)
                    }
                  >
                    {props.canAppointOwner && (
                      <NativeSelectOption value='owner'>
                        {t('Owner')}
                      </NativeSelectOption>
                    )}
                    <NativeSelectOption value='member'>
                      {t('Member')}
                    </NativeSelectOption>
                    <NativeSelectOption value='admin'>
                      {t('Admin')}
                    </NativeSelectOption>
                    <NativeSelectOption value='auditor'>
                      {t('Auditor')}
                    </NativeSelectOption>
                  </NativeSelect>
                </label>
                <Button type='submit'>{t('Add member')}</Button>
              </form>
              <div className='overflow-x-auto'>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>{t('User ID')}</TableHead>
                      <TableHead>{t('Role')}</TableHead>
                      <TableHead>{t('Status')}</TableHead>
                      <TableHead />
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {props.members.map((membership) => (
                      <TableRow key={membership.id}>
                        <TableCell>{membership.user_id}</TableCell>
                        <TableCell>
                          <NativeSelect
                            value={membership.role}
                            onChange={(event) =>
                              props.onUpdateMember({
                                ...membership,
                                role: event.target.value,
                              })
                            }
                          >
                            {props.canAppointOwner && (
                              <NativeSelectOption value='owner'>
                                {t('Owner')}
                              </NativeSelectOption>
                            )}
                            <NativeSelectOption value='admin'>
                              {t('Admin')}
                            </NativeSelectOption>
                            <NativeSelectOption value='member'>
                              {t('Member')}
                            </NativeSelectOption>
                            <NativeSelectOption value='auditor'>
                              {t('Auditor')}
                            </NativeSelectOption>
                          </NativeSelect>
                        </TableCell>
                        <TableCell>
                          {membership.status === 1
                            ? t('Active')
                            : t('Inactive')}
                        </TableCell>
                        <TableCell>
                          <Button
                            size='sm'
                            variant='outline'
                            type='button'
                            onClick={() =>
                              props.onUpdateMember({
                                ...membership,
                                status: membership.status === 1 ? 2 : 1,
                              })
                            }
                          >
                            {membership.status === 1
                              ? t('Disable')
                              : t('Enable')}
                          </Button>
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </div>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle>{t('Invitations')}</CardTitle>
            </CardHeader>
            <CardContent className='space-y-4'>
              <form
                className='flex flex-wrap items-end gap-2'
                onSubmit={props.onCreateInvitation}
              >
                <label className='grid gap-1 text-sm'>
                  {t('Max uses')}
                  <Input
                    required
                    inputMode='numeric'
                    value={props.invitationMaxUses}
                    onChange={(event) =>
                      props.onInvitationMaxUsesChange(event.target.value)
                    }
                  />
                </label>
                <Button type='submit'>{t('Create invitation')}</Button>
              </form>
              <div className='overflow-x-auto'>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>{t('ID')}</TableHead>
                      <TableHead>{t('Status')}</TableHead>
                      <TableHead>{t('Max uses')}</TableHead>
                      <TableHead>{t('Used')}</TableHead>
                      <TableHead />
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {props.invitations.map((invitation) => (
                      <TableRow key={invitation.id}>
                        <TableCell>{invitation.id}</TableCell>
                        <TableCell>
                          {invitation.status === 1
                            ? t('Enabled')
                            : t('Disabled')}
                        </TableCell>
                        <TableCell>
                          {invitation.max_uses || t('Unlimited')}
                        </TableCell>
                        <TableCell>{invitation.used_count}</TableCell>
                        <TableCell>
                          <Button
                            size='sm'
                            variant='outline'
                            type='button'
                            onClick={() => props.onToggleInvitation(invitation)}
                          >
                            {invitation.status === 1
                              ? t('Disable invitation')
                              : t('Enable invitation')}
                          </Button>
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </div>
            </CardContent>
          </Card>
        </>
      )}
      {!props.selectedEnterpriseId && (
        <Card>
          <CardContent className='text-muted-foreground pt-6 text-sm'>
            {t('No enterprise selected')}
          </CardContent>
        </Card>
      )}
    </div>
  )
}

function EnterpriseRankingTable(props: { data: EnterpriseRanking[] }) {
  const { t } = useTranslation()
  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('Enterprise consumption rankings')}</CardTitle>
      </CardHeader>
      <CardContent>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t('Rank')}</TableHead>
              <TableHead>{t('Enterprise')}</TableHead>
              <TableHead>{t('Net consumption')}</TableHead>
              <TableHead>{t('Tokens')}</TableHead>
              <TableHead>{t('Requests')}</TableHead>
              <TableHead>{t('Active users')}</TableHead>
              <TableHead>{t('Growth')}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {props.data.map((row) => (
              <TableRow key={row.enterprise_id}>
                <TableCell>{row.rank}</TableCell>
                <TableCell>{row.name}</TableCell>
                <TableCell>{formatNumber(row.net_quota)}</TableCell>
                <TableCell>{formatNumber(row.total_tokens)}</TableCell>
                <TableCell>{formatNumber(row.request_count)}</TableCell>
                <TableCell>{formatNumber(row.active_users)}</TableCell>
                <TableCell>{formatGrowth(row.growth_pct)}</TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </CardContent>
    </Card>
  )
}

function MemberRankingTable(props: {
  data: {
    rank: number
    username: string
    net_quota: number
    total_tokens: number
    request_count: number
    growth_pct: number
  }[]
}) {
  const { t } = useTranslation()
  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('Member consumption rankings')}</CardTitle>
      </CardHeader>
      <CardContent>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t('Rank')}</TableHead>
              <TableHead>{t('Member')}</TableHead>
              <TableHead>{t('Net consumption')}</TableHead>
              <TableHead>{t('Tokens')}</TableHead>
              <TableHead>{t('Requests')}</TableHead>
              <TableHead>{t('Growth')}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {props.data.map((row) => (
              <TableRow key={row.username}>
                <TableCell>{row.rank}</TableCell>
                <TableCell>{row.username}</TableCell>
                <TableCell>{formatNumber(row.net_quota)}</TableCell>
                <TableCell>{formatNumber(row.total_tokens)}</TableCell>
                <TableCell>{formatNumber(row.request_count)}</TableCell>
                <TableCell>{formatGrowth(row.growth_pct)}</TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </CardContent>
    </Card>
  )
}
