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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'

import { Assets } from '../assets'

describe('Asset library', () => {
  it('requires a writable ready group and preserves request identity after an uncertain create', async () => {
    vi.spyOn(api, 'get').mockImplementation(async (url) => ({
      data: {
        success: true,
        data: {
          total: 2,
          items: String(url).endsWith('/groups')
            ? [
                {
                  id: 'aig_private',
                  name: 'My group',
                  scope: 'private',
                  state: 'active',
                  can_manage: true,
                },
                {
                  id: 'aig_public',
                  name: 'Public group',
                  scope: 'public',
                  state: 'active',
                  can_manage: false,
                },
              ]
            : [],
        },
      },
    }))
    const posts = vi
      .spyOn(api, 'post')
      .mockRejectedValue(new Error('Outcome requires reconciliation'))
    const client = new QueryClient({
      defaultOptions: {
        queries: { retry: false },
        mutations: { retry: false },
      },
    })
    render(
      <QueryClientProvider client={client}>
        <Assets />
      </QueryClientProvider>
    )
    await screen.findByRole('option', { name: 'My group · private · active' })
    fireEvent.change(screen.getByLabelText('Name'), {
      target: { value: 'Fox' },
    })
    fireEvent.change(screen.getByLabelText('Material URL'), {
      target: { value: 'https://cdn.example/fox.png' },
    })
    const submit = screen.getByRole('button', { name: 'Add material' })
    expect(submit).toBeDisabled()
    fireEvent.change(screen.getByLabelText('Material group'), {
      target: { value: 'aig_public' },
    })
    expect(submit).toBeDisabled()
    fireEvent.change(screen.getByLabelText('Material group'), {
      target: { value: 'aig_private' },
    })
    fireEvent.click(submit)
    await waitFor(() => expect(posts).toHaveBeenCalledTimes(1))
    await waitFor(() => expect(submit).not.toBeDisabled())
    fireEvent.click(submit)
    await waitFor(() => expect(posts).toHaveBeenCalledTimes(2))
    const first = posts.mock.calls[0][1] as { request_id: string }
    const second = posts.mock.calls[1][1] as { request_id: string }
    expect(first.request_id).toBeTruthy()
    expect(second.request_id).toBe(first.request_id)
    fireEvent.change(screen.getByLabelText('Name'), {
      target: { value: 'A different material' },
    })
    await waitFor(() => expect(submit).not.toBeDisabled())
    fireEvent.click(submit)
    await waitFor(() => expect(posts).toHaveBeenCalledTimes(3))
    expect(
      (posts.mock.calls[2][1] as { request_id: string }).request_id
    ).not.toBe(first.request_id)
  })

  it('paginates materials and resets the page when searching', async () => {
    const reads = vi
      .spyOn(api, 'get')
      .mockImplementation(async (url) => ({
        data: {
          success: true,
          data: { total: String(url).endsWith('/groups') ? 0 : 48, items: [] },
        },
      }))
    const client = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    })
    render(
      <QueryClientProvider client={client}>
        <Assets />
      </QueryClientProvider>
    )
    fireEvent.click(
      await screen.findByRole('button', { name: 'Go to next page' })
    )
    await waitFor(() =>
      expect(reads).toHaveBeenCalledWith(
        '/api/zhiqi-assets/assets',
        expect.objectContaining({ params: expect.objectContaining({ p: 2 }) })
      )
    )
    fireEvent.change(screen.getByLabelText('Search materials'), {
      target: { value: 'fox' },
    })
    fireEvent.click(screen.getByRole('button', { name: /^Search$/ }))
    await waitFor(() =>
      expect(reads).toHaveBeenCalledWith(
        '/api/zhiqi-assets/assets',
        expect.objectContaining({
          params: expect.objectContaining({ p: 1, keyword: 'fox' }),
        })
      )
    )
  })
})
