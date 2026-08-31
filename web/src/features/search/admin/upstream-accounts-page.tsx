/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  CircleAlert,
  CircleDollarSign,
  HeartPulse,
  KeyRound,
  Network,
  Pause,
  Pencil,
  Play,
  Plus,
  RefreshCw,
  Trash2,
} from 'lucide-react'
import { useState, type ReactNode } from 'react'
import { Controller, useForm, type UseFormReturn } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

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
  AlertDialogTrigger,
} from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty'
import {
  Field,
  FieldDescription,
  FieldError,
  FieldGroup,
  FieldLabel,
} from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { handleServerError } from '@/lib/handle-server-error'

import {
  createSearchUpstreamAccount,
  deleteSearchUpstreamAccount,
  fetchSearchUpstreamAccounts,
  testSearchUpstreamAccount,
  updateSearchUpstreamAccount,
  type SearchUpstreamAccount,
} from '../api'
import {
  createUpstreamAccountSchema,
  getUpstreamProviderDefaultURL,
  getUpstreamAccountServerFormError,
  UPSTREAM_PROVIDER_OPTIONS,
  UPSTREAM_ACCOUNT_FORM_DEFAULTS,
  updateUpstreamAccountSchema,
  type UpstreamAccountFormValues,
} from '../lib/upstream-account-form'
import { formatMoney } from '../money'
import { SearchAdminShell } from './search-admin-shell'

export function SearchAdminUpstreamAccountsPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [dialogOpen, setDialogOpen] = useState(false)
  const [editingAccount, setEditingAccount] =
    useState<SearchUpstreamAccount | null>(null)
  const createForm = useForm<UpstreamAccountFormValues>({
    resolver: zodResolver(createUpstreamAccountSchema),
    defaultValues: UPSTREAM_ACCOUNT_FORM_DEFAULTS,
  })
  const editForm = useForm<UpstreamAccountFormValues>({
    resolver: zodResolver(updateUpstreamAccountSchema),
    defaultValues: UPSTREAM_ACCOUNT_FORM_DEFAULTS,
  })
  const accountsQuery = useQuery({
    queryKey: ['search-admin-upstream-accounts'],
    queryFn: fetchSearchUpstreamAccounts,
  })
  const createMutation = useMutation({
    mutationFn: createSearchUpstreamAccount,
    onSuccess: () => {
      setDialogOpen(false)
      createForm.reset(UPSTREAM_ACCOUNT_FORM_DEFAULTS)
      toast.success(t('Upstream account connected'))
      void queryClient.invalidateQueries({
        queryKey: ['search-admin-upstream-accounts'],
      })
    },
    onError: (error) => {
      setAccountFormServerError(createForm, error, t)
    },
  })
  const testMutation = useMutation({
    mutationFn: testSearchUpstreamAccount,
    onSuccess: () => {
      toast.success(t('Health check completed'))
      void queryClient.invalidateQueries({
        queryKey: ['search-admin-upstream-accounts'],
      })
    },
    onError: handleServerError,
  })
  const updateMutation = useMutation({
    mutationFn: updateSearchUpstreamAccount,
    onSuccess: () => {
      setEditingAccount(null)
      editForm.reset(UPSTREAM_ACCOUNT_FORM_DEFAULTS)
      toast.success(t('Upstream account updated'))
      void queryClient.invalidateQueries({
        queryKey: ['search-admin-upstream-accounts'],
      })
    },
  })
  const deleteMutation = useMutation({
    mutationFn: deleteSearchUpstreamAccount,
    onSuccess: () => {
      toast.success(t('Upstream account deleted'))
      void queryClient.invalidateQueries({
        queryKey: ['search-admin-upstream-accounts'],
      })
    },
    onError: handleServerError,
  })

  const accounts = accountsQuery.data || []
  const healthyCount = accounts.filter(
    (account) => account.status === 'healthy'
  ).length
  const totalWeight = accounts.reduce(
    (total, account) => total + account.weight,
    0
  )
  const openCreateDialog = () => {
    createForm.reset(UPSTREAM_ACCOUNT_FORM_DEFAULTS)
    setDialogOpen(true)
  }
  const openEditDialog = (account: SearchUpstreamAccount) => {
    editForm.reset(accountEditInput(account))
    setEditingAccount(account)
  }

  let accountContent: ReactNode
  if (accountsQuery.isLoading) {
    accountContent = <AccountsSkeleton />
  } else if (accountsQuery.isError) {
    accountContent = (
      <Empty className='min-h-56 rounded-none border-0'>
        <EmptyHeader>
          <EmptyMedia variant='icon'>
            <CircleAlert aria-hidden='true' />
          </EmptyMedia>
          <EmptyTitle>{t('Failed to load upstream accounts')}</EmptyTitle>
          <EmptyDescription>
            {t('Check administrator permissions and try again.')}
          </EmptyDescription>
        </EmptyHeader>
        <Button variant='outline' onClick={() => void accountsQuery.refetch()}>
          {t('Retry')}
        </Button>
      </Empty>
    )
  } else if (accounts.length === 0) {
    accountContent = (
      <Empty className='min-h-56 rounded-none border-0'>
        <EmptyHeader>
          <EmptyMedia variant='icon'>
            <Network aria-hidden='true' />
          </EmptyMedia>
          <EmptyTitle>{t('No upstream accounts')}</EmptyTitle>
          <EmptyDescription>
            {t('Connect a provider account before enabling capabilities.')}
          </EmptyDescription>
        </EmptyHeader>
        <Button variant='outline' onClick={openCreateDialog}>
          {t('Connect account')}
        </Button>
      </Empty>
    )
  } else {
    accountContent = (
      <>
        <div className='hidden overflow-x-auto md:block'>
          <AccountTable
            accounts={accounts}
            testingId={testMutation.isPending ? testMutation.variables : null}
            updatingId={
              updateMutation.isPending ? updateMutation.variables.id : null
            }
            deletingId={
              deleteMutation.isPending ? deleteMutation.variables : null
            }
            onTest={(id) => testMutation.mutate(id)}
            onToggle={(account) =>
              updateMutation.mutate(accountStatusUpdate(account), {
                onError: handleServerError,
              })
            }
            onEdit={openEditDialog}
            onDelete={(id) => deleteMutation.mutate(id)}
            t={t}
          />
        </div>
        <div className='divide-y md:hidden'>
          {accounts.map((account) => (
            <AccountCard
              key={account.id}
              account={account}
              testing={
                testMutation.isPending && testMutation.variables === account.id
              }
              updating={
                updateMutation.isPending &&
                updateMutation.variables.id === account.id
              }
              deleting={
                deleteMutation.isPending &&
                deleteMutation.variables === account.id
              }
              onTest={() => testMutation.mutate(account.id)}
              onToggle={() =>
                updateMutation.mutate(accountStatusUpdate(account), {
                  onError: handleServerError,
                })
              }
              onEdit={() => openEditDialog(account)}
              onDelete={() => deleteMutation.mutate(account.id)}
              t={t}
            />
          ))}
        </div>
      </>
    )
  }

  return (
    <SearchAdminShell
      title={t('vSearch key management')}
      description={t(
        'Configure the JustOneAPI and TikHub accounts used by the vSearch runtime. API keys are never shown after connection.'
      )}
      action={
        <Button onClick={openCreateDialog}>
          <Plus data-icon='inline-start' aria-hidden='true' />
          {t('Connect account')}
        </Button>
      }
    >
      <div className='grid gap-3 sm:grid-cols-2 xl:grid-cols-4'>
        <AccountMetric
          icon={Network}
          label={t('Connected accounts')}
          value={
            accountsQuery.isLoading ? '—' : accounts.length.toLocaleString()
          }
        />
        <AccountMetric
          icon={HeartPulse}
          label={t('Healthy accounts')}
          value={accountsQuery.isLoading ? '—' : healthyCount.toLocaleString()}
        />
        <AccountMetric
          icon={CircleDollarSign}
          label={t('Available balance')}
          value={
            accountsQuery.isLoading ? '—' : formatTotalAccountBalance(accounts)
          }
        />
        <AccountMetric
          icon={KeyRound}
          label={t('Routing weight')}
          value={accountsQuery.isLoading ? '—' : totalWeight.toLocaleString()}
        />
      </div>

      <Card>
        <CardContent className='p-0'>
          <div className='flex flex-col gap-3 border-b p-4 sm:flex-row sm:items-center sm:justify-between sm:p-5'>
            <div>
              <h2 className='font-semibold'>{t('Upstream accounts')}</h2>
              <p className='text-muted-foreground mt-1 text-sm'>
                {t(
                  'Manage health, routing pool, weight, and provider balance.'
                )}
              </p>
            </div>
            <Button
              variant='outline'
              onClick={() => void accountsQuery.refetch()}
              disabled={accountsQuery.isFetching}
            >
              <RefreshCw
                data-icon='inline-start'
                className={
                  accountsQuery.isFetching ? 'animate-spin' : undefined
                }
                aria-hidden='true'
              />
              {t('Refresh')}
            </Button>
          </div>
          {accountContent}
        </CardContent>
      </Card>

      <Dialog
        open={dialogOpen}
        onOpenChange={(open) => {
          if (!open && createMutation.isPending) return
          setDialogOpen(open)
          if (!open) createForm.reset(UPSTREAM_ACCOUNT_FORM_DEFAULTS)
        }}
      >
        <DialogContent className='sm:max-w-lg'>
          <DialogHeader>
            <DialogTitle>{t('Connect upstream provider account')}</DialogTitle>
            <DialogDescription>
              {t(
                'The key is stored encrypted and the account is enabled immediately. TikHub supports health checks; JustOneAPI does not provide a non-billable probe.'
              )}
            </DialogDescription>
          </DialogHeader>
          <form
            id='search-upstream-account-form'
            noValidate
            onSubmit={createForm.handleSubmit((values) => {
              createMutation.mutate(values)
            })}
          >
            <AccountFormFields
              idPrefix='create'
              form={createForm}
              secretRequired
            />
          </form>
          <DialogFooter>
            <Button
              variant='outline'
              type='button'
              onClick={() => {
                setDialogOpen(false)
                createForm.reset(UPSTREAM_ACCOUNT_FORM_DEFAULTS)
              }}
              disabled={createMutation.isPending}
            >
              {t('Cancel')}
            </Button>
            <Button
              type='submit'
              form='search-upstream-account-form'
              disabled={createMutation.isPending}
            >
              {createMutation.isPending
                ? t('Connecting…')
                : t('Connect account')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog
        open={editingAccount !== null}
        onOpenChange={(open) => {
          if (!open && !updateMutation.isPending) {
            setEditingAccount(null)
            editForm.reset(UPSTREAM_ACCOUNT_FORM_DEFAULTS)
          }
        }}
      >
        <DialogContent className='sm:max-w-lg'>
          <DialogHeader>
            <DialogTitle>
              {t('Edit {{title}}', { title: editingAccount?.name || '' })}
            </DialogTitle>
            <DialogDescription>
              {t('Leave blank to keep the existing key')}
            </DialogDescription>
          </DialogHeader>
          <form
            id='search-upstream-account-edit-form'
            noValidate
            onSubmit={editForm.handleSubmit((values) => {
              if (!editingAccount) return
              updateMutation.mutate(
                {
                  ...values,
                  id: editingAccount.id,
                },
                {
                  onError: (error) => {
                    setAccountFormServerError(editForm, error, t)
                  },
                }
              )
            })}
          >
            <AccountFormFields
              idPrefix='edit'
              form={editForm}
              secretRequired={false}
            />
          </form>
          <DialogFooter>
            <Button
              variant='outline'
              type='button'
              onClick={() => {
                setEditingAccount(null)
                editForm.reset(UPSTREAM_ACCOUNT_FORM_DEFAULTS)
              }}
              disabled={updateMutation.isPending}
            >
              {t('Cancel')}
            </Button>
            <Button
              type='submit'
              form='search-upstream-account-edit-form'
              disabled={updateMutation.isPending}
            >
              {updateMutation.isPending ? t('Saving…') : t('Save changes')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </SearchAdminShell>
  )
}

function AccountFormFields(props: {
  idPrefix: string
  form: UseFormReturn<UpstreamAccountFormValues>
  secretRequired: boolean
}) {
  const { t } = useTranslation()
  const idPrefix = `search-upstream-${props.idPrefix}`
  const errors = props.form.formState.errors
  return (
    <FieldGroup>
      {errors.root?.server && (
        <Field data-invalid>
          <FieldError
            errors={[{ message: t(String(errors.root.server.message)) }]}
          />
        </Field>
      )}
      <Field data-invalid={Boolean(errors.name)}>
        <FieldLabel htmlFor={`${idPrefix}-name`}>
          {t('Account name')}
        </FieldLabel>
        <Input
          id={`${idPrefix}-name`}
          aria-invalid={Boolean(errors.name)}
          placeholder={t('e.g. Primary production account')}
          {...props.form.register('name')}
        />
        <FieldError
          errors={
            errors.name
              ? [{ message: t(String(errors.name.message)) }]
              : undefined
          }
        />
      </Field>
      <Controller
        control={props.form.control}
        name='provider'
        render={({ field }) => (
          <Field data-invalid={Boolean(errors.provider)}>
            <FieldLabel htmlFor={`${idPrefix}-provider`}>
              {t('Provider')}
            </FieldLabel>
            <Select
              items={UPSTREAM_PROVIDER_OPTIONS}
              value={field.value}
              onValueChange={(value) => {
                if (!value) return
                field.onChange(value)
                props.form.setValue(
                  'base_url',
                  getUpstreamProviderDefaultURL(value),
                  { shouldDirty: true, shouldValidate: true }
                )
              }}
            >
              <SelectTrigger
                id={`${idPrefix}-provider`}
                className='w-full'
                aria-invalid={Boolean(errors.provider)}
              >
                <SelectValue />
              </SelectTrigger>
              <SelectContent alignItemWithTrigger={false}>
                <SelectGroup>
                  {UPSTREAM_PROVIDER_OPTIONS.map((option) => (
                    <SelectItem key={option.value} value={option.value}>
                      {option.label}
                    </SelectItem>
                  ))}
                </SelectGroup>
              </SelectContent>
            </Select>
            <FieldDescription>
              {t('Choose the upstream service for this account.')}
            </FieldDescription>
            <FieldError
              errors={
                errors.provider
                  ? [{ message: t(String(errors.provider.message)) }]
                  : undefined
              }
            />
          </Field>
        )}
      />
      <Field data-invalid={Boolean(errors.api_key)}>
        <FieldLabel htmlFor={`${idPrefix}-key`}>
          {t('Provider API key')}
        </FieldLabel>
        <Input
          id={`${idPrefix}-key`}
          type='password'
          autoComplete='new-password'
          aria-invalid={Boolean(errors.api_key)}
          placeholder={t('Paste the upstream provider API key')}
          {...props.form.register('api_key')}
        />
        <FieldDescription>
          {props.secretRequired
            ? t('Only a masked prefix is shown after connection.')
            : t('Leave blank to keep the existing key')}
        </FieldDescription>
        <FieldError
          errors={
            errors.api_key
              ? [{ message: t(String(errors.api_key.message)) }]
              : undefined
          }
        />
      </Field>
      <Field data-invalid={Boolean(errors.base_url)}>
        <FieldLabel htmlFor={`${idPrefix}-base-url`}>
          {t('Provider API base URL')}
        </FieldLabel>
        <Input
          id={`${idPrefix}-base-url`}
          type='url'
          aria-invalid={Boolean(errors.base_url)}
          {...props.form.register('base_url')}
        />
        <FieldDescription>
          {t(
            'Use the default URL unless your provider account uses a custom endpoint.'
          )}
        </FieldDescription>
        <FieldError
          errors={
            errors.base_url
              ? [{ message: t(String(errors.base_url.message)) }]
              : undefined
          }
        />
      </Field>
      <FieldGroup className='grid gap-4 sm:grid-cols-3'>
        <Field data-invalid={Boolean(errors.pool_id)}>
          <FieldLabel htmlFor={`${idPrefix}-pool`}>{t('Pool ID')}</FieldLabel>
          <Input
            id={`${idPrefix}-pool`}
            type='number'
            min={0}
            aria-invalid={Boolean(errors.pool_id)}
            {...props.form.register('pool_id', { valueAsNumber: true })}
          />
          <FieldError
            errors={
              errors.pool_id
                ? [{ message: t(String(errors.pool_id.message)) }]
                : undefined
            }
          />
        </Field>
        <Field data-invalid={Boolean(errors.weight)}>
          <FieldLabel htmlFor={`${idPrefix}-weight`}>{t('Weight')}</FieldLabel>
          <Input
            id={`${idPrefix}-weight`}
            type='number'
            min={1}
            max={100}
            aria-invalid={Boolean(errors.weight)}
            {...props.form.register('weight', { valueAsNumber: true })}
          />
          <FieldError
            errors={
              errors.weight
                ? [{ message: t(String(errors.weight.message)) }]
                : undefined
            }
          />
        </Field>
        <Field data-invalid={Boolean(errors.priority)}>
          <FieldLabel htmlFor={`${idPrefix}-priority`}>
            {t('Priority')}
          </FieldLabel>
          <Input
            id={`${idPrefix}-priority`}
            type='number'
            min={0}
            aria-invalid={Boolean(errors.priority)}
            {...props.form.register('priority', { valueAsNumber: true })}
          />
          <FieldError
            errors={
              errors.priority
                ? [{ message: t(String(errors.priority.message)) }]
                : undefined
            }
          />
        </Field>
      </FieldGroup>
    </FieldGroup>
  )
}

function AccountMetric(props: {
  icon: typeof Network
  label: string
  value: string
}) {
  const Icon = props.icon
  return (
    <Card>
      <CardContent className='p-4'>
        <div className='flex items-center justify-between gap-3'>
          <span className='text-muted-foreground text-xs font-medium'>
            {props.label}
          </span>
          <span className='bg-primary/10 text-primary flex size-8 items-center justify-center rounded-lg'>
            <Icon className='size-4' aria-hidden='true' />
          </span>
        </div>
        <div className='mt-4 font-mono text-2xl font-semibold'>
          {props.value}
        </div>
      </CardContent>
    </Card>
  )
}

function AccountTable(props: {
  accounts: SearchUpstreamAccount[]
  testingId: number | null
  updatingId: number | null
  deletingId: number | null
  onTest: (id: number) => void
  onToggle: (account: SearchUpstreamAccount) => void
  onEdit: (account: SearchUpstreamAccount) => void
  onDelete: (id: number) => void
  t: (key: string) => string
}) {
  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>{props.t('Account')}</TableHead>
          <TableHead>{props.t('Plan')}</TableHead>
          <TableHead>{props.t('Balance')}</TableHead>
          <TableHead>{props.t('Routing')}</TableHead>
          <TableHead>{props.t('Status')}</TableHead>
          <TableHead className='text-right'>{props.t('Actions')}</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {props.accounts.map((account) => (
          <TableRow key={account.id}>
            <TableCell>
              <div className='font-medium'>{account.name}</div>
              <div className='text-muted-foreground mt-1 font-mono text-xs'>
                {upstreamProviderLabel(account.provider)} · {account.key_prefix}
              </div>
            </TableCell>
            <TableCell>{account.plan || '—'}</TableCell>
            <TableCell>{formatAccountBalance(account)}</TableCell>
            <TableCell className='text-muted-foreground text-xs'>
              {account.pool} · {props.t('Weight')} {account.weight} ·{' '}
              {props.t('Priority')} {account.priority}
            </TableCell>
            <TableCell>
              <AccountStatus account={account} t={props.t} />
            </TableCell>
            <TableCell className='text-right'>
              <AccountActions
                account={account}
                testing={props.testingId === account.id}
                updating={props.updatingId === account.id}
                deleting={props.deletingId === account.id}
                busy={
                  props.testingId !== null ||
                  props.updatingId !== null ||
                  props.deletingId !== null
                }
                onTest={() => props.onTest(account.id)}
                onToggle={() => props.onToggle(account)}
                onEdit={() => props.onEdit(account)}
                onDelete={() => props.onDelete(account.id)}
                t={props.t}
              />
            </TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  )
}

function AccountCard(props: {
  account: SearchUpstreamAccount
  testing: boolean
  updating: boolean
  deleting: boolean
  onTest: () => void
  onToggle: () => void
  onEdit: () => void
  onDelete: () => void
  t: (key: string) => string
}) {
  const { account, t } = props
  return (
    <article className='space-y-4 p-4'>
      <div className='flex items-start justify-between gap-3'>
        <div className='min-w-0'>
          <p className='truncate font-medium'>{account.name}</p>
          <p className='text-muted-foreground mt-1 truncate font-mono text-xs'>
            {upstreamProviderLabel(account.provider)} · {account.key_prefix}
          </p>
        </div>
        <AccountStatus account={account} t={t} />
      </div>
      <div className='text-muted-foreground grid grid-cols-2 gap-2 text-xs'>
        <span>{account.plan || '—'}</span>
        <span className='text-right'>{formatAccountBalance(account)}</span>
        <span>{account.pool}</span>
        <span className='text-right'>
          {t('Weight')} {account.weight}
        </span>
      </div>
      <AccountActions
        account={account}
        testing={props.testing}
        updating={props.updating}
        deleting={props.deleting}
        busy={props.testing || props.updating || props.deleting}
        onTest={props.onTest}
        onToggle={props.onToggle}
        onEdit={props.onEdit}
        onDelete={props.onDelete}
        t={t}
      />
    </article>
  )
}

function AccountActions(props: {
  account: SearchUpstreamAccount
  testing: boolean
  updating: boolean
  deleting: boolean
  busy: boolean
  onTest: () => void
  onToggle: () => void
  onEdit: () => void
  onDelete: () => void
  t: (key: string, options?: Record<string, unknown>) => string
}) {
  const paused = props.account.status === 'paused'
  return (
    <div className='flex flex-wrap justify-end gap-2'>
      <Button
        variant='outline'
        size='sm'
        onClick={props.onTest}
        disabled={props.busy}
      >
        <RefreshCw
          data-icon='inline-start'
          className={props.testing ? 'animate-spin' : undefined}
          aria-hidden='true'
        />
        {props.t('Test')}
      </Button>
      <Button
        variant='outline'
        size='sm'
        onClick={props.onEdit}
        disabled={props.busy}
      >
        <Pencil data-icon='inline-start' aria-hidden='true' />
        {props.t('Edit')}
      </Button>
      <Button
        variant='outline'
        size='sm'
        onClick={props.onToggle}
        disabled={props.busy}
      >
        {paused ? (
          <Play data-icon='inline-start' aria-hidden='true' />
        ) : (
          <Pause data-icon='inline-start' aria-hidden='true' />
        )}
        {props.updating
          ? props.t('Saving…')
          : props.t(paused ? 'Enable' : 'Pause')}
      </Button>
      <AlertDialog>
        <AlertDialogTrigger
          render={<Button variant='destructive' size='sm' />}
          disabled={props.busy}
        >
          <Trash2 data-icon='inline-start' aria-hidden='true' />
          {props.t('Delete')}
        </AlertDialogTrigger>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {props.t('Delete upstream account?')}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {props.t(
                'Delete {{name}}? Capabilities routed only through this account will become unavailable.',
                { name: props.account.name }
              )}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{props.t('Cancel')}</AlertDialogCancel>
            <AlertDialogAction
              variant='destructive'
              onClick={props.onDelete}
              disabled={props.deleting}
            >
              {props.deleting ? props.t('Deleting…') : props.t('Delete')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}

function accountStatusUpdate(account: SearchUpstreamAccount) {
  return {
    id: account.id,
    provider: account.provider,
    name: account.name,
    base_url: account.base_url,
    pool_id: account.pool_id,
    weight: account.weight,
    priority: account.priority,
    status:
      account.status === 'paused' ? ('healthy' as const) : ('paused' as const),
  }
}

function accountEditInput(
  account: SearchUpstreamAccount
): UpstreamAccountFormValues {
  return {
    provider: account.provider,
    name: account.name,
    api_key: '',
    base_url: account.base_url,
    pool_id: account.pool_id,
    weight: account.weight,
    priority: account.priority,
    status: account.status,
  }
}

function upstreamProviderLabel(provider: SearchUpstreamAccount['provider']) {
  return (
    UPSTREAM_PROVIDER_OPTIONS.find((option) => option.value === provider)
      ?.label || provider
  )
}

function formatAccountBalance(account: SearchUpstreamAccount) {
  if (!account.balance_currency.trim()) return '—'
  return formatMoney(
    { micros: account.balance_micros, amount: account.balance },
    account.balance_currency
  )
}

function formatTotalAccountBalance(accounts: SearchUpstreamAccount[]) {
  if (accounts.length === 0) return '—'
  const totals = new Map<string, number>()
  for (const account of accounts) {
    const currency = account.balance_currency.trim().toUpperCase()
    if (!currency) continue
    totals.set(currency, (totals.get(currency) || 0) + account.balance_micros)
  }
  if (totals.size === 0) return '—'
  return [...totals.entries()]
    .sort(([left], [right]) => left.localeCompare(right))
    .map(([currency, micros]) => formatMoney({ micros }, currency))
    .join(' · ')
}

function setAccountFormServerError(
  form: UseFormReturn<UpstreamAccountFormValues>,
  error: unknown,
  t: (key: string) => string
) {
  const serverError = getUpstreamAccountServerFormError(error)
  form.setError(
    serverError.field,
    { type: 'server', message: t(serverError.messageKey) },
    { shouldFocus: serverError.field !== 'root.server' }
  )
}

function AccountStatus(props: {
  account: SearchUpstreamAccount
  t: (key: string) => string
}) {
  const healthy = props.account.status === 'healthy'
  const warning = props.account.status === 'warning'
  let variant: 'neutral' | 'success' | 'warning' = 'neutral'
  if (healthy) variant = 'success'
  if (warning) variant = 'warning'
  return (
    <StatusBadge
      label={props.t(props.account.status)}
      variant={variant}
      copyable={false}
    />
  )
}

function AccountsSkeleton() {
  return (
    <div className='space-y-4 p-5' aria-hidden='true'>
      {Array.from({ length: 4 }, (_, index) => (
        <Skeleton key={index} className='h-10 w-full' />
      ))}
    </div>
  )
}
