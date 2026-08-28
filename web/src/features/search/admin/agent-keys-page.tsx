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
*/
import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { KeyRound, Loader2, ServerCog } from 'lucide-react'
import { useState, type ReactNode } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'

import { CopyButton } from '@/components/copy-button'
import { EmptyState } from '@/components/empty-state'
import { ErrorState } from '@/components/error-state'
import { StatusBadge } from '@/components/status-badge'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Field,
  FieldContent,
  FieldError,
  FieldGroup,
  FieldLabel,
  FieldLegend,
  FieldSet,
  FieldTitle,
} from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import { handleServerError } from '@/lib/handle-server-error'
import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'

import {
  createAdminSearchAgentKey,
  createAdminSearchInstallToken,
  fetchAdminSearchAgentKeys,
  fetchSearchAgentKeyOwnerCandidates,
  revokeAdminSearchAgentKey,
  type SearchAgentKeyApiRecord,
} from '../api'
import {
  adminSearchAgentKeySchema,
  getAdminSearchAgentKeyFormDefaults,
  type AdminSearchAgentKeyFormValues,
} from '../lib/admin-agent-key-form'
import { SEARCH_CAPABILITY_GROUPS } from '../types'
import { SearchAdminShell } from './search-admin-shell'

type InstallPlatform = 'macOS / Linux' | 'Windows PowerShell'

export function SearchAdminAgentKeysPage() {
  const { t } = useTranslation()
  const user = useAuthStore((state) => state.auth.user)
  const managedEnterpriseId =
    user?.role === ROLE.SUPER_ADMIN ? undefined : user?.enterprise?.id
  const queryClient = useQueryClient()
  const [createdSecret, setCreatedSecret] = useState('')
  const [revokeKeyId, setRevokeKeyId] = useState<number | null>(null)
  const [installCommand, setInstallCommand] = useState('')
  const [installKeyId, setInstallKeyId] = useState<number | null>(null)
  const [installKeyLabel, setInstallKeyLabel] = useState('')
  const [installPlatform, setInstallPlatform] =
    useState<InstallPlatform>('macOS / Linux')
  const form = useForm<AdminSearchAgentKeyFormValues>({
    resolver: zodResolver(adminSearchAgentKeySchema),
    defaultValues: getAdminSearchAgentKeyFormDefaults(user?.id),
  })
  const ownerCandidatesQuery = useQuery({
    queryKey: [
      'search-admin-agent-key-owners',
      managedEnterpriseId ?? 'platform',
    ],
    queryFn: () => fetchSearchAgentKeyOwnerCandidates(managedEnterpriseId),
    enabled: Boolean(user),
  })
  const keysQuery = useQuery({
    queryKey: ['search-admin-agent-keys'],
    queryFn: fetchAdminSearchAgentKeys,
  })
  const createMutation = useMutation({
    mutationFn: (input: AdminSearchAgentKeyFormValues) =>
      createAdminSearchAgentKey(input),
    onSuccess: (created) => {
      setCreatedSecret(created.secret)
      form.reset(getAdminSearchAgentKeyFormDefaults(created.user_id))
      void queryClient.invalidateQueries({
        queryKey: ['search-admin-agent-keys'],
      })
    },
    onError: handleServerError,
  })
  const revokeMutation = useMutation({
    mutationFn: (id: number) => revokeAdminSearchAgentKey(id),
    onSuccess: (_, revokedId) => {
      setRevokeKeyId(null)
      if (installKeyId === revokedId) {
        setInstallCommand('')
        setInstallKeyId(null)
        setInstallKeyLabel('')
      }
      void queryClient.invalidateQueries({
        queryKey: ['search-admin-agent-keys'],
      })
    },
    onError: handleServerError,
  })
  const installMutation = useMutation({
    mutationFn: async (input: {
      keyId: number
      keyLabel: string
      platform: InstallPlatform
    }) => ({
      keyLabel: input.keyLabel,
      platform: input.platform,
      token: await createAdminSearchInstallToken(input.keyId),
    }),
    onMutate: () => {
      setInstallCommand('')
      setInstallKeyId(null)
      setInstallKeyLabel('')
    },
    onSuccess: ({ keyLabel, platform, token }, input) => {
      const origin = window.location.origin.replace(/\/$/, '')
      setInstallKeyId(input.keyId)
      setInstallKeyLabel(keyLabel)
      setInstallPlatform(platform)
      setInstallCommand(
        platform === 'macOS / Linux'
          ? `curl -fsSL '${origin}/install.sh' | bash -s -- --token '${token.token}' --origin '${origin}'`
          : `& ([scriptblock]::Create((Invoke-RestMethod '${origin}/install.ps1'))) -Token '${token.token}' -Origin '${origin}'`
      )
    },
    onError: handleServerError,
  })

  const scopes = form.watch('scopes')
  const keys = keysQuery.data ?? []
  const showInstallCommand =
    installCommand !== '' &&
    keys.some((key) => key.id === installKeyId && key.status === 'active')
  let ownerFieldError: string | null = null
  if (ownerCandidatesQuery.isError) {
    ownerFieldError = t('Failed to load users')
  } else if (form.formState.errors.user_id?.message) {
    ownerFieldError = t(form.formState.errors.user_id.message)
  }

  let keyList: ReactNode
  if (keysQuery.isLoading) {
    keyList = (
      <div className='text-muted-foreground flex min-h-40 items-center justify-center gap-2 text-sm'>
        <Loader2 className='size-4 animate-spin' aria-hidden='true' />
        {t('Loading…')}
      </div>
    )
  } else if (keysQuery.isError) {
    keyList = (
      <ErrorState
        className='min-h-52'
        description={t('Failed to load vSearch keys')}
        onRetry={() => void keysQuery.refetch()}
      />
    )
  } else if (keys.length === 0) {
    keyList = (
      <EmptyState
        icon={KeyRound}
        className='min-h-52'
        title={t('No vSearch keys yet')}
        description={t(
          'Review and revoke vSearch credentials within your enterprise.'
        )}
      />
    )
  } else {
    keyList = (
      <div className='space-y-3'>
        {keys.map((key) => (
          <div
            key={key.id}
            data-search-admin-key
            className='flex min-w-0 flex-col gap-4 rounded-lg border p-4 md:flex-row md:items-center md:justify-between'
          >
            <div className='min-w-0'>
              <div className='flex min-w-0 flex-wrap items-center gap-2'>
                <span className='truncate font-medium'>{key.label}</span>
                <StatusBadge
                  label={t(key.status)}
                  variant={statusVariant(key.status)}
                  copyable={false}
                />
              </div>
              <p className='text-muted-foreground mt-1 text-sm'>
                {t('Owner')}: {key.owner || `${t('User')} #${key.user_id}`}
              </p>
              <p className='text-muted-foreground mt-1 font-mono text-xs'>
                {key.prefix}••••••••
              </p>
              <p className='text-muted-foreground mt-1 text-xs break-words'>
                {key.scopes
                  .map((scope) => {
                    const group = SEARCH_CAPABILITY_GROUPS.find(
                      (item) => item.id === scope
                    )
                    return group ? t(group.label) : scope
                  })
                  .join(' · ')}
              </p>
            </div>
            <div className='flex flex-wrap gap-2'>
              {key.status === 'active' && (
                <>
                  <Button
                    variant='outline'
                    size='sm'
                    disabled={installMutation.isPending}
                    onClick={() =>
                      installMutation.mutate({
                        keyId: key.id,
                        keyLabel: key.label,
                        platform: 'macOS / Linux',
                      })
                    }
                  >
                    {t('macOS / Linux')}
                  </Button>
                  <Button
                    variant='outline'
                    size='sm'
                    disabled={installMutation.isPending}
                    onClick={() =>
                      installMutation.mutate({
                        keyId: key.id,
                        keyLabel: key.label,
                        platform: 'Windows PowerShell',
                      })
                    }
                  >
                    {t('Windows PowerShell')}
                  </Button>
                </>
              )}
              {key.status !== 'revoked' && (
                <Button
                  variant='outline'
                  size='sm'
                  disabled={revokeMutation.isPending}
                  onClick={() => setRevokeKeyId(key.id)}
                >
                  {t('Revoke key')}
                </Button>
              )}
            </div>
          </div>
        ))}
      </div>
    )
  }

  return (
    <SearchAdminShell
      title={t('vSearch keys')}
      description={t(
        'Enterprise managers can manage vSearch access without seeing upstream provider secrets. Platform administrators can additionally configure connectors and routing.'
      )}
      action={
        <>
          {user?.role === ROLE.SUPER_ADMIN && (
            <Button
              variant='outline'
              render={<Link to='/search/admin/upstream-accounts' />}
            >
              <ServerCog data-icon='inline-start' aria-hidden='true' />
              {t('Upstream accounts')}
            </Button>
          )}
          <Button
            type='submit'
            form='search-admin-agent-key-form'
            disabled={
              createMutation.isPending ||
              ownerCandidatesQuery.isLoading ||
              ownerCandidatesQuery.isError ||
              ownerCandidatesQuery.data?.length === 0
            }
          >
            {createMutation.isPending && (
              <Loader2
                className='size-4 animate-spin'
                data-icon='inline-start'
                aria-hidden='true'
              />
            )}
            {t('Create vSearch key')}
          </Button>
        </>
      }
    >
      <Card>
        <CardHeader>
          <CardTitle>{t('Create a vSearch key')}</CardTitle>
          <CardDescription>
            {t(
              'Create a dedicated vSearch key for search access. This page will not show or modify model API Keys.'
            )}
          </CardDescription>
        </CardHeader>
        <CardContent>
          <form
            id='search-admin-agent-key-form'
            onSubmit={form.handleSubmit((values) =>
              createMutation.mutate(values)
            )}
          >
            <FieldGroup>
              <div className='grid gap-5 md:grid-cols-2'>
                <Field
                  data-invalid={Boolean(form.formState.errors.user_id)}
                  data-disabled={ownerCandidatesQuery.isLoading}
                >
                  <FieldLabel htmlFor='search-admin-agent-key-owner'>
                    {t('Owner')}
                  </FieldLabel>
                  <NativeSelect
                    id='search-admin-agent-key-owner'
                    className='w-full'
                    disabled={ownerCandidatesQuery.isLoading}
                    aria-invalid={Boolean(form.formState.errors.user_id)}
                    {...form.register('user_id', { valueAsNumber: true })}
                  >
                    <NativeSelectOption value=''>
                      {ownerCandidatesQuery.isLoading
                        ? t('Loading...')
                        : t('Select a user')}
                    </NativeSelectOption>
                    {ownerCandidatesQuery.data?.map((candidate) => (
                      <NativeSelectOption
                        key={candidate.id}
                        value={candidate.id}
                      >
                        {candidate.display_name || candidate.username} ·{' '}
                        {candidate.username} (ID {candidate.id})
                      </NativeSelectOption>
                    ))}
                  </NativeSelect>
                  <FieldError>{ownerFieldError}</FieldError>
                </Field>
                <Field data-invalid={Boolean(form.formState.errors.name)}>
                  <FieldLabel htmlFor='search-admin-agent-key-name'>
                    {t('Key name')}
                  </FieldLabel>
                  <Input
                    id='search-admin-agent-key-name'
                    autoComplete='off'
                    placeholder={t('e.g. research-bot…')}
                    aria-invalid={Boolean(form.formState.errors.name)}
                    {...form.register('name')}
                  />
                  <FieldError>
                    {form.formState.errors.name?.message
                      ? t(form.formState.errors.name.message)
                      : null}
                  </FieldError>
                </Field>
              </div>

              <FieldSet>
                <FieldLegend variant='label'>
                  {t('Capability scopes')}
                </FieldLegend>
                <FieldGroup
                  data-slot='checkbox-group'
                  className='grid gap-3 sm:grid-cols-2 xl:grid-cols-4'
                >
                  {SEARCH_CAPABILITY_GROUPS.map((group) => {
                    const checked = scopes.includes(group.id)
                    return (
                      <FieldLabel key={group.id}>
                        <Field orientation='horizontal'>
                          <Checkbox
                            id={`search-admin-agent-key-scope-${group.id}`}
                            checked={checked}
                            aria-invalid={Boolean(form.formState.errors.scopes)}
                            onCheckedChange={(nextChecked) =>
                              form.setValue(
                                'scopes',
                                nextChecked
                                  ? [...scopes, group.id]
                                  : scopes.filter(
                                      (scope) => scope !== group.id
                                    ),
                                { shouldDirty: true, shouldValidate: true }
                              )
                            }
                          />
                          <FieldContent>
                            <FieldTitle>{t(group.label)}</FieldTitle>
                          </FieldContent>
                        </Field>
                      </FieldLabel>
                    )
                  })}
                </FieldGroup>
                <FieldError>
                  {form.formState.errors.scopes?.message
                    ? t(form.formState.errors.scopes.message)
                    : null}
                </FieldError>
              </FieldSet>
            </FieldGroup>
          </form>

          {createdSecret && (
            <div
              className='bg-success/10 mt-5 rounded-lg border border-green-500/30 p-4 text-sm'
              aria-live='polite'
            >
              <p className='font-medium'>{t('vSearch key created')}</p>
              <p className='text-muted-foreground mt-1 text-xs'>
                {t(
                  'Copy this secret now. It will not be shown again after you leave this page.'
                )}
              </p>
              <div className='bg-background/70 mt-3 flex flex-col gap-2 rounded-lg border p-3 sm:flex-row sm:items-center'>
                <code
                  className='min-w-0 flex-1 font-mono text-xs break-all'
                  translate='no'
                >
                  {createdSecret}
                </code>
                <CopyButton
                  value={createdSecret}
                  variant='outline'
                  size='sm'
                  tooltip={t('Copy AgentKey secret')}
                  successTooltip={t('AgentKey secret copied')}
                >
                  {t('Copy')}
                </CopyButton>
              </div>
            </div>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t('Manage vSearch keys')}</CardTitle>
          <CardDescription>
            {t(
              'vSearch keys are separate from model API Keys. The same user and enterprise permissions still apply.'
            )}
          </CardDescription>
        </CardHeader>
        <CardContent className='space-y-4'>
          {keyList}
          {showInstallCommand && (
            <div
              className='bg-muted/40 rounded-lg border p-4'
              aria-live='polite'
            >
              <div className='flex flex-wrap items-start justify-between gap-3'>
                <div>
                  <p className='font-medium'>{t('Generate install command')}</p>
                  <p className='text-muted-foreground mt-1 text-xs'>
                    {t(installPlatform)} ·{' '}
                    {t(
                      'The command expires in 15 minutes and can be used once.'
                    )}
                  </p>
                  <p className='text-muted-foreground mt-1 text-xs'>
                    {t('Key name')}: {installKeyLabel}
                  </p>
                </div>
                <CopyButton
                  value={installCommand}
                  variant='outline'
                  size='sm'
                  tooltip={t('Copy')}
                  successTooltip={t('Copied')}
                >
                  {t('Copy')}
                </CopyButton>
              </div>
              <pre className='bg-background mt-3 overflow-x-auto rounded-md border p-3 text-xs'>
                <code translate='no'>{installCommand}</code>
              </pre>
            </div>
          )}
        </CardContent>
      </Card>

      <AlertDialog
        open={revokeKeyId !== null}
        onOpenChange={(open) => !open && setRevokeKeyId(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('Revoke AgentKey?')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t(
                'Requests using this AgentKey will stop working immediately. This action cannot be undone.'
              )}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={revokeMutation.isPending}>
              {t('Cancel')}
            </AlertDialogCancel>
            <AlertDialogAction
              variant='destructive'
              disabled={revokeMutation.isPending}
              onClick={() => {
                if (revokeKeyId !== null) revokeMutation.mutate(revokeKeyId)
              }}
            >
              {revokeMutation.isPending ? t('Revoking…') : t('Revoke key')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </SearchAdminShell>
  )
}

function statusVariant(status: SearchAgentKeyApiRecord['status']) {
  if (status === 'active') return 'success' as const
  return 'neutral' as const
}
