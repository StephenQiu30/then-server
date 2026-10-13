import type { ReactNode } from 'react'
import { AuthLayout } from '@/layout/auth-layout'

export default function AccountAccessLayout({
  children,
}: {
  children: ReactNode
}) {
  return <AuthLayout>{children}</AuthLayout>
}
