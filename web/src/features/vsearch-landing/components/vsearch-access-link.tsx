/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { Link } from '@tanstack/react-router'

interface VSearchAccessLinkProps {
  children: React.ReactNode
  className: string
  isAuthenticated: boolean
}

export function VSearchAccessLink(props: VSearchAccessLinkProps) {
  if (props.isAuthenticated) {
    return (
      <Link to='/search/catalog' className={props.className}>
        {props.children}
      </Link>
    )
  }

  return (
    <Link
      to='/sign-in'
      search={{ redirect: '/search/catalog' }}
      className={props.className}
    >
      {props.children}
    </Link>
  )
}
