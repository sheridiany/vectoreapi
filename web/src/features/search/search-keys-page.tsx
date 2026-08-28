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
import {
  BookOpen,
  KeyRound,
  Loader2,
  ShieldCheck,
  Terminal,
} from 'lucide-react'
import { useMemo, useState, type ReactNode } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { z } from 'zod'

import { CopyButton } from '@/components/copy-button'
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
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { handleServerError } from '@/lib/handle-server-error'

import {
  createSearchAgentKey,
  createSearchInstallToken,
  fetchSearchAgentKeys,
  revokeSearchAgentKey,
  type SearchAgentKeyApiRecord,
} from './api'
import { SearchShell } from './components/search-shell'
import { SEARCH_CAPABILITY_GROUPS } from './types'

const searchAgentKeySchema = z.object({
  name: z
    .string()
    .trim()
    .min(1, 'Key name is required')
    .max(64, 'Key name is too long'),
  scopes: z.array(z.string()).min(1, 'Select at least one capability'),
})
type SearchAgentKeyForm = z.infer<typeof searchAgentKeySchema>

export function SearchKeysPage() {
  const { t } = useTranslation()
  const [createdKeyId, setCreatedKeyId] = useState<number | null>(null)
  const [createdSecret, setCreatedSecret] = useState<string | null>(null)
  const [revokeKeyId, setRevokeKeyId] = useState<number | null>(null)
  const [installCommand, setInstallCommand] = useState('')
  const [installKeyId, setInstallKeyId] = useState<number | null>(null)
  const [installKeyLabel, setInstallKeyLabel] = useState('')
  const [installPlatform, setInstallPlatform] = useState<
    'macOS / Linux' | 'Windows PowerShell'
  >('macOS / Linux')
  const queryClient = useQueryClient()
  const form = useForm<SearchAgentKeyForm>({
    resolver: zodResolver(searchAgentKeySchema),
    defaultValues: {
      name: '',
      scopes: SEARCH_CAPABILITY_GROUPS.map((group) => group.id),
    },
  })
  const keysQuery = useQuery({
    queryKey: ['search-agent-keys'],
    queryFn: fetchSearchAgentKeys,
  })
  const createMutation = useMutation({
    mutationFn: ({ name, scopes }: SearchAgentKeyForm) =>
      createSearchAgentKey(name, scopes),
    onSuccess: (created) => {
      setCreatedKeyId(created.id)
      setCreatedSecret(created.secret)
      form.reset()
      void queryClient.invalidateQueries({ queryKey: ['search-agent-keys'] })
    },
    onError: handleServerError,
  })
  const revokeMutation = useMutation({
    mutationFn: revokeSearchAgentKey,
    onSuccess: (_, revokedId) => {
      setRevokeKeyId(null)
      if (installKeyId === revokedId) {
        setInstallCommand('')
        setInstallKeyId(null)
        setInstallKeyLabel('')
      }
      void queryClient.invalidateQueries({ queryKey: ['search-agent-keys'] })
    },
    onError: handleServerError,
  })
  const installMutation = useMutation({
    mutationFn: async (input: {
      keyId: number
      keyLabel: string
      platform: 'macOS / Linux' | 'Windows PowerShell'
    }) => ({
      keyLabel: input.keyLabel,
      platform: input.platform,
      token: await createSearchInstallToken(input.keyId),
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

  const scopeLabels = useMemo<Map<string, string>>(
    () =>
      new Map(
        SEARCH_CAPABILITY_GROUPS.map((group) => [group.id, t(group.label)])
      ),
    [t]
  )

  const keys = keysQuery.data || []
  const showInstallCommand =
    installCommand !== '' &&
    keys.some((key) => key.id === installKeyId && key.status === 'active')

  let keyList: ReactNode
  if (keysQuery.isLoading) {
    keyList = (
      <div className='text-muted-foreground flex items-center gap-2 text-sm'>
        <Loader2 className='size-4 animate-spin' />
        {t('Loading…')}
      </div>
    )
  } else if (keysQuery.isError) {
    keyList = (
      <p className='text-destructive text-sm' role='alert'>
        {t('Failed to load vSearch keys')}
      </p>
    )
  } else if (keys.length === 0) {
    keyList = (
      <p className='text-muted-foreground text-sm'>
        {t('No vSearch keys yet')}
      </p>
    )
  } else {
    keyList = (
      <div className='space-y-3'>
        {keys.map((key) => (
          <div
            key={key.id}
            data-search-key
            className='flex min-w-0 flex-col gap-3 rounded-lg border p-4 sm:flex-row sm:items-center sm:justify-between'
          >
            <div className='min-w-0'>
              <div className='flex min-w-0 items-center gap-2'>
                <span className='truncate font-medium'>{key.label}</span>
                <StatusBadge
                  label={t(key.status)}
                  variant={statusVariant(key.status)}
                  copyable={false}
                />
              </div>
              <p className='text-muted-foreground mt-1 font-mono text-xs'>
                {key.prefix}••••••••
              </p>
              <p className='text-muted-foreground mt-1 text-xs break-words'>
                {key.scopes
                  .map((scope) => scopeLabels.get(scope) || scope)
                  .join(' · ')}
              </p>
            </div>
            <div className='flex flex-wrap gap-2'>
              {key.status === 'active' && (
                <>
                  <Button
                    variant='outline'
                    size='sm'
                    onClick={() =>
                      installMutation.mutate({
                        keyId: key.id,
                        keyLabel: key.label,
                        platform: 'macOS / Linux',
                      })
                    }
                    disabled={installMutation.isPending}
                  >
                    {t('macOS / Linux')}
                  </Button>
                  <Button
                    variant='outline'
                    size='sm'
                    onClick={() =>
                      installMutation.mutate({
                        keyId: key.id,
                        keyLabel: key.label,
                        platform: 'Windows PowerShell',
                      })
                    }
                    disabled={installMutation.isPending}
                  >
                    {t('Windows PowerShell')}
                  </Button>
                </>
              )}
              {key.status !== 'revoked' && (
                <Button
                  variant='outline'
                  size='sm'
                  onClick={() => setRevokeKeyId(key.id)}
                  disabled={revokeMutation.isPending}
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
    <SearchShell
      title={t('vSearch keys')}
      description={t(
        'vSearch keys are separate from model API Keys. The same user and enterprise permissions still apply.'
      )}
      action={
        <Button
          type='submit'
          form='search-agent-key-form'
          disabled={createMutation.isPending}
        >
          {t('Create vSearch key')}
        </Button>
      }
    >
      <Card>
        <CardContent className='p-5 sm:p-7'>
          <div className='flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between'>
            <div className='flex gap-3'>
              <div className='bg-primary/10 text-primary flex size-11 shrink-0 items-center justify-center rounded-xl'>
                <KeyRound className='size-5' aria-hidden='true' />
              </div>
              <div>
                <h2 className='text-lg font-semibold'>
                  {t('Create a vSearch key')}
                </h2>
                <p className='text-muted-foreground mt-1 max-w-2xl text-sm leading-6'>
                  {t(
                    'Create a dedicated vSearch key for search access. This page will not show or modify model API Keys.'
                  )}
                </p>
              </div>
            </div>
            <StatusBadge
              label={t('Managed by gate-relay')}
              variant='success'
              copyable={false}
            />
          </div>

          <form
            id='search-agent-key-form'
            className='mt-6 grid gap-4 rounded-lg border p-4 sm:grid-cols-[1fr_auto] sm:items-end'
            onSubmit={form.handleSubmit((values) =>
              createMutation.mutate(values)
            )}
          >
            <label
              className='text-sm font-medium'
              htmlFor='search-agent-key-name'
            >
              {t('Key name')}
              <Input
                id='search-agent-key-name'
                autoComplete='off'
                placeholder={t('e.g. research-bot…')}
                {...form.register('name')}
              />
              {form.formState.errors.name && (
                <span className='text-destructive mt-1 block text-xs'>
                  {t(form.formState.errors.name.message || 'Invalid key name')}
                </span>
              )}
            </label>
            <div className='flex flex-wrap gap-2'>
              {SEARCH_CAPABILITY_GROUPS.map((group) => {
                const enabled = scopes.includes(group.id)
                return (
                  <button
                    key={group.id}
                    type='button'
                    aria-pressed={enabled}
                    className={`focus-visible:ring-ring rounded-md border px-3 py-2 text-xs focus-visible:ring-2 focus-visible:outline-none ${enabled ? 'border-primary bg-primary/10 text-primary' : 'text-muted-foreground'}`}
                    onClick={() =>
                      form.setValue(
                        'scopes',
                        enabled
                          ? scopes.filter((scope) => scope !== group.id)
                          : [...scopes, group.id],
                        { shouldValidate: true }
                      )
                    }
                  >
                    {t(group.label)}
                  </button>
                )
              })}
            </div>
          </form>
          {form.formState.errors.scopes && (
            <p className='text-destructive mt-3 text-sm' role='alert'>
              {t(
                form.formState.errors.scopes.message ||
                  'Select at least one capability'
              )}
            </p>
          )}
          {createdKeyId && createdSecret && (
            <div
              className='bg-success/10 mt-4 rounded-lg border border-green-500/30 p-4 text-sm'
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
          <CardTitle>{t('Your vSearch keys')}</CardTitle>
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
                    <span>{t('Key name')}:</span>{' '}
                    <span className='font-medium'>{installKeyLabel}</span>
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
              <p className='text-muted-foreground mt-3 text-xs leading-5'>
                {t(
                  'Running this command rotates this vSearch key in place. The previous key stops working immediately.'
                )}
              </p>
            </div>
          )}
        </CardContent>
      </Card>

      <div className='grid gap-4 sm:grid-cols-2'>
        <Card>
          <CardContent className='p-5'>
            <div className='flex items-center gap-2'>
              <BookOpen className='text-primary size-5' aria-hidden='true' />
              <h3 className='font-semibold'>{t('How to use vSearch')}</h3>
            </div>
            <dl className='mt-4 space-y-3 text-sm'>
              <Definition label={t('Step 1')} value={t('Create a key')} />
              <Definition
                label={t('Step 2')}
                value={t('Copy AgentKey secret')}
              />
              <Definition
                label={t('Step 3')}
                value={t('Call a vSearch capability')}
              />
            </dl>
            <p className='text-muted-foreground mt-4 flex items-start gap-2 text-sm leading-6'>
              <Terminal
                className='text-primary mt-0.5 size-4 shrink-0'
                aria-hidden='true'
              />
              {t(
                'Use this key only with vSearch endpoints and keep the full secret out of client-side code.'
              )}
            </p>
          </CardContent>
        </Card>
        <Card>
          <CardContent className='p-5'>
            <div className='flex items-center gap-2'>
              <ShieldCheck className='text-success size-5' aria-hidden='true' />
              <h3 className='font-semibold'>
                {t('Available vSearch capabilities')}
              </h3>
            </div>
            <div className='mt-4 flex flex-wrap gap-2'>
              {SEARCH_CAPABILITY_GROUPS.map((group) => (
                <StatusBadge
                  key={group.id}
                  label={t(group.label)}
                  variant='info'
                  copyable={false}
                />
              ))}
            </div>
            <p className='text-muted-foreground mt-4 text-sm leading-6'>
              {t(
                'Each key receives explicit capabilities. Upstream connector secrets remain an admin-only setting.'
              )}
            </p>
          </CardContent>
        </Card>
      </div>

      <p className='text-muted-foreground text-sm'>
        {t('Looking for model credentials?')}{' '}
        <Link className='text-primary font-medium hover:underline' to='/keys'>
          {t('Open model API Keys')}
        </Link>
      </p>

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
    </SearchShell>
  )
}

function statusVariant(status: SearchAgentKeyApiRecord['status']) {
  if (status === 'active') return 'success' as const
  return 'neutral' as const
}

function Definition(props: { label: string; value: string }) {
  return (
    <div className='flex items-center justify-between gap-4'>
      <dt className='text-muted-foreground'>{props.label}</dt>
      <dd className='text-right font-medium'>{props.value}</dd>
    </div>
  )
}
