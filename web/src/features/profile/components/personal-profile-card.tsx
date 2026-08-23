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
import { Loader2, UserRound } from 'lucide-react'
import { useEffect, useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import { TitledCard } from '@/components/ui/titled-card'

import type { UpdateUserRequest, UserProfile } from '../types'

interface PersonalProfileCardProps {
  profile: UserProfile | null
  loading: boolean
  onSave: (data: UpdateUserRequest) => Promise<boolean>
}

export function PersonalProfileCard({
  profile,
  loading,
  onSave,
}: PersonalProfileCardProps) {
  const { t } = useTranslation()
  const [displayName, setDisplayName] = useState('')
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    setDisplayName(profile?.display_name ?? '')
  }, [profile?.display_name])

  if (loading) {
    return (
      <TitledCard
        title={t('Personal profile')}
        description={t('Set the name your enterprise sees in usage rankings.')}
        icon={<UserRound className='h-4 w-4' />}
        iconTone='info'
        disableHoverEffect
      >
        <Skeleton className='h-8 w-full' />
      </TitledCard>
    )
  }

  if (!profile) return null

  const normalizedName = displayName.trim()
  const canSave =
    normalizedName.length > 0 && normalizedName !== profile.display_name

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!canSave || saving) return

    setSaving(true)
    try {
      await onSave({ display_name: normalizedName })
    } finally {
      setSaving(false)
    }
  }

  return (
    <TitledCard
      title={t('Personal profile')}
      description={t('Set the name your enterprise sees in usage rankings.')}
      icon={<UserRound className='h-4 w-4' />}
      iconTone='info'
      disableHoverEffect
    >
      <form
        className='flex flex-col gap-3 sm:flex-row sm:items-end'
        onSubmit={handleSubmit}
      >
        <div className='min-w-0 flex-1 space-y-1.5'>
          <label className='text-sm font-medium' htmlFor='profile-display-name'>
            {t('Display name')}
          </label>
          <Input
            id='profile-display-name'
            value={displayName}
            onChange={(event) => setDisplayName(event.target.value)}
            placeholder={profile.username}
            maxLength={20}
            autoComplete='name'
            disabled={saving}
          />
        </div>
        <Button
          type='submit'
          disabled={!canSave || saving}
          className='sm:min-w-20'
        >
          {saving && <Loader2 className='animate-spin' />}
          {t('Save')}
        </Button>
      </form>
    </TitledCard>
  )
}
