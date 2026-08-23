import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { createInstance } from 'i18next'
import { I18nextProvider, initReactI18next } from 'react-i18next'
import { describe, expect, test, vi } from 'vitest'

import type { UserProfile } from '../../types'
import { PersonalProfileCard } from '../personal-profile-card'

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: {
    en: {
      translation: {
        'Personal profile': 'Personal profile',
        'Set the name your enterprise sees in usage rankings.':
          'Set the name your enterprise sees in usage rankings.',
        'Display name': 'Display name',
        Save: 'Save',
      },
    },
  },
})

const profile: UserProfile = {
  id: 7,
  username: 'alice-login',
  display_name: '',
  role: 1,
  group: 'default',
  quota: 0,
  used_quota: 0,
  request_count: 0,
  status: 1,
  aff_count: 0,
  aff_quota: 0,
  aff_history_quota: 0,
  created_time: 0,
}

describe('PersonalProfileCard', () => {
  test('saves the name entered by the user', async () => {
    const onSave = vi.fn().mockResolvedValue(true)
    const user = userEvent.setup()

    render(
      <I18nextProvider i18n={i18n}>
        <PersonalProfileCard
          profile={profile}
          loading={false}
          onSave={onSave}
        />
      </I18nextProvider>
    )

    const input = screen.getByLabelText('Display name')
    await user.type(input, 'Alice Chen')
    await user.click(screen.getByRole('button', { name: 'Save' }))

    expect(onSave).toHaveBeenCalledWith({ display_name: 'Alice Chen' })
  })
})
