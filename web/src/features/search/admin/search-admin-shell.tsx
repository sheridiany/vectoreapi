/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import type { ReactNode } from 'react'

import { SectionPageLayout } from '@/components/layout'

export function SearchAdminShell(props: {
  title: string
  description?: string
  action?: ReactNode
  children: ReactNode
}) {
  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{props.title}</SectionPageLayout.Title>
      <SectionPageLayout.Actions>{props.action}</SectionPageLayout.Actions>
      <SectionPageLayout.Content>
        <div className='mx-auto w-full max-w-[1440px] min-w-0 space-y-5 overflow-x-hidden'>
          {props.description && (
            <p className='text-muted-foreground max-w-3xl text-sm leading-6'>
              {props.description}
            </p>
          )}
          {props.children}
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
