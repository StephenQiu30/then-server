'use client'

import { useQueryClient } from '@tanstack/react-query'
import { useMemo } from 'react'
import { deleteCurrentUser, updateCurrentUser } from '@/api/account'
import {
  createSession,
  deleteSession,
  registerAccount,
} from '@/api/authentication'
import { currentUserKey } from '@/hooks/account/use-current-user'

export function useAccountActions() {
  const queryClient = useQueryClient()
  return useMemo(
    () => ({
      async register(body: API.RegisterAccountRequest) {
        const result = await registerAccount(body)
        queryClient.clear()
        queryClient.setQueryData(currentUserKey, result.user)
      },
      async login(body: API.CreateSessionRequest) {
        const result = await createSession(body)
        queryClient.clear()
        queryClient.setQueryData(currentUserKey, result.user)
      },
      async update(body: Parameters<typeof updateCurrentUser>[0]) {
        const user = await updateCurrentUser(body)
        queryClient.setQueryData(currentUserKey, user)
      },
      async logout() {
        await deleteSession()
        queryClient.clear()
      },
      async remove(): Promise<API.AccountDeletionResponse> {
        const receipt =
          (await deleteCurrentUser()) as API.AccountDeletionResponse
        queryClient.clear()
        return receipt
      },
      clearCurrent() {
        queryClient.clear()
      },
    }),
    [queryClient],
  )
}
