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
import {
  fireEvent,
  render,
  screen,
  waitFor,
  cleanup,
} from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { Workbench } from '../workbench'

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
})

describe('Media workbench', () => {
  it('requires a local key, model and prompt and queries the created task without resubmitting', async () => {
    const fetcher = vi
      .fn()
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ id: 'task_local', status: 'queued' }), {
          status: 200,
        })
      )
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({ id: 'task_local', status: 'completed' }),
          { status: 200 }
        )
      )
    vi.stubGlobal('fetch', fetcher)
    const client = new QueryClient({
      defaultOptions: {
        queries: { retry: false },
        mutations: { retry: false },
      },
    })
    render(
      <QueryClientProvider client={client}>
        <Workbench />
      </QueryClientProvider>
    )
    expect(
      screen.getByRole('button', { name: 'Create generation task' })
    ).toBeDisabled()
    fireEvent.change(screen.getByLabelText('Local API key'), {
      target: { value: 'sk-local-fixture' },
    })
    fireEvent.change(screen.getByLabelText('Model'), {
      target: { value: 'zhiqi-video' },
    })
    fireEvent.change(screen.getByLabelText('Prompt'), {
      target: { value: 'A fox' },
    })
    fireEvent.change(screen.getByLabelText('Additional parameters (JSON)'), {
      target: { value: '{"generate_audio":false,"seed":0}' },
    })
    fireEvent.click(
      screen.getByRole('button', { name: 'Create generation task' })
    )
    await waitFor(() =>
      expect(screen.getByLabelText('Task ID')).toHaveValue('task_local')
    )
    expect(fetcher).toHaveBeenCalledTimes(1)
    const init = fetcher.mock.calls[0][1] as RequestInit
    expect(init.credentials).toBe('omit')
    expect(
      (init.headers as Record<string, string>)['Idempotency-Key']
    ).toBeTruthy()
    expect(JSON.parse(String(init.body))).toMatchObject({
      model: 'zhiqi-video',
      prompt: 'A fox',
      seed: 0,
      generate_audio: false,
    })
    expect(localStorage.getItem('zhiqi-api-key')).toBeNull()
    fireEvent.click(screen.getByRole('button', { name: 'Query task' }))
    await waitFor(() => expect(fetcher).toHaveBeenCalledTimes(2))
    expect(fetcher.mock.calls[1][0]).toBe('/v1/videos/task_local')
    expect(fetcher.mock.calls[1][1].method).toBe('GET')
    fireEvent.change(screen.getByLabelText('Media type'), {
      target: { value: 'image' },
    })
    expect(screen.getByLabelText('Task ID')).toHaveValue('')
  })

  it('rejects malformed extra parameters before a paid request and does not retry upstream errors', async () => {
    const fetcher = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ error: { message: 'busy' } }), {
        status: 503,
      })
    )
    vi.stubGlobal('fetch', fetcher)
    const client = new QueryClient({
      defaultOptions: { mutations: { retry: false } },
    })
    render(
      <QueryClientProvider client={client}>
        <Workbench />
      </QueryClientProvider>
    )
    for (const [label, value] of [
      ['Local API key', 'fixture'],
      ['Model', 'video'],
      ['Prompt', 'fox'],
      ['Additional parameters (JSON)', '[]'],
    ]) {
      fireEvent.change(screen.getByLabelText(label), { target: { value } })
    }
    fireEvent.click(
      screen.getByRole('button', { name: 'Create generation task' })
    )
    await waitFor(() =>
      expect(
        screen.getByRole('button', { name: 'Create generation task' })
      ).not.toBeDisabled()
    )
    expect(fetcher).not.toHaveBeenCalled()
    fireEvent.change(screen.getByLabelText('Additional parameters (JSON)'), {
      target: { value: '{}' },
    })
    fireEvent.click(
      screen.getByRole('button', { name: 'Create generation task' })
    )
    await waitFor(() => expect(fetcher).toHaveBeenCalledTimes(1))
    await waitFor(() =>
      expect(
        screen.getByRole('button', { name: 'Create generation task' })
      ).not.toBeDisabled()
    )
    expect(fetcher).toHaveBeenCalledTimes(1)
    const firstKey = fetcher.mock.calls[0][1].headers['Idempotency-Key']
    fireEvent.click(
      screen.getByRole('button', { name: 'Create generation task' })
    )
    await waitFor(() => expect(fetcher).toHaveBeenCalledTimes(2))
    expect(fetcher.mock.calls[1][1].headers['Idempotency-Key']).toBe(firstKey)
  })
})
