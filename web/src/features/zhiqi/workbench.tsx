import { useMutation } from '@tanstack/react-query'
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
import { nanoid } from 'nanoid'
import { useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { CopyButton } from '@/components/copy-button'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import { Textarea } from '@/components/ui/textarea'

export function Workbench() {
  const { t } = useTranslation()
  const [key, setKey] = useState('')
  const [mode, setMode] = useState('video')
  const [model, setModel] = useState('')
  const [prompt, setPrompt] = useState('')
  const [options, setOptions] = useState('{}')
  const [taskId, setTaskId] = useState('')
  const [output, setOutput] = useState<Record<string, unknown> | null>(null)
  const idempotency = useRef({ signature: '', key: '' })
  const request = async (path: string, body?: unknown, requestKey?: string) => {
    const response = await fetch(path, {
      method: body ? 'POST' : 'GET',
      headers: {
        Authorization: `Bearer ${key.trim()}`,
        ...(body ? { 'Content-Type': 'application/json' } : {}),
        ...(requestKey ? { 'Idempotency-Key': requestKey } : {}),
      },
      body: body ? JSON.stringify(body) : undefined,
      credentials: 'omit',
    })
    const result = (await response.json()) as Record<string, unknown>
    if (!response.ok) throw new Error(JSON.stringify(result.error || result))
    return result
  }
  const submit = useMutation({
    retry: false,
    mutationFn: async () => {
      const extra: unknown = JSON.parse(options)
      if (!extra || typeof extra !== 'object' || Array.isArray(extra)) {
        throw new Error('Additional parameters must be an object')
      }
      const signature = JSON.stringify({ mode, model, prompt, extra, key })
      if (idempotency.current.signature !== signature) {
        idempotency.current = { signature, key: nanoid(32) }
      }
      return request(
        {
          video: '/v1/videos',
          image: '/v1/images/generations/async',
          image_sync: '/v1/images/generations',
        }[mode] || '/v1/videos',
        { ...extra, model, prompt },
        mode === 'image_sync' ? undefined : idempotency.current.key
      )
    },
    onSuccess: (result) => {
      setOutput(result)
      if (typeof result.id === 'string') setTaskId(result.id)
      idempotency.current = { signature: '', key: '' }
    },
  })
  const query = useMutation({
    mutationFn: () =>
      request(
        `${mode === 'video' ? '/v1/videos/' : '/v1/images/generations/'}${encodeURIComponent(taskId)}`
      ),
    onSuccess: setOutput,
  })
  const download = useMutation({
    mutationFn: async () => {
      const response = await fetch(
        `/v1/videos/${encodeURIComponent(taskId)}/content`,
        {
          headers: { Authorization: `Bearer ${key.trim()}` },
          credentials: 'omit',
        }
      )
      if (!response.ok) throw new Error(t('Video download failed'))
      const address = URL.createObjectURL(await response.blob())
      const link = document.createElement('a')
      link.href = address
      link.download = `${taskId}.mp4`
      link.click()
      setTimeout(() => URL.revokeObjectURL(address), 1000)
    },
  })
  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('Media workbench')}</CardTitle>
      </CardHeader>
      <CardContent className='space-y-4'>
        <p className='text-muted-foreground text-sm'>
          {t(
            'Use a local API key. Submitting creates a billable task. Keys stay in this page memory only.'
          )}
        </p>
        <Label htmlFor='media-key'>{t('Local API key')}</Label>
        <Input
          id='media-key'
          type='password'
          autoComplete='off'
          value={key}
          onChange={(event) => setKey(event.target.value)}
        />
        <div className='grid gap-4 md:grid-cols-2'>
          <div className='space-y-2'>
            <Label htmlFor='media-mode'>{t('Media type')}</Label>
            <NativeSelect
              id='media-mode'
              value={mode}
              onChange={(event) => {
                setMode(event.target.value)
                setTaskId('')
                setOutput(null)
              }}
            >
              <NativeSelectOption value='video'>
                {t('Video')}
              </NativeSelectOption>
              <NativeSelectOption value='image'>
                {t('Asynchronous image')}
              </NativeSelectOption>
              <NativeSelectOption value='image_sync'>
                {t('Synchronous image')}
              </NativeSelectOption>
            </NativeSelect>
          </div>
          <div className='space-y-2'>
            <Label htmlFor='media-model'>{t('Model')}</Label>
            <Input
              id='media-model'
              value={model}
              onChange={(event) => setModel(event.target.value)}
            />
          </div>
        </div>
        <Label htmlFor='media-prompt'>{t('Prompt')}</Label>
        <Textarea
          id='media-prompt'
          value={prompt}
          onChange={(event) => setPrompt(event.target.value)}
        />
        <Label htmlFor='media-options'>
          {t('Additional parameters (JSON)')}
        </Label>
        <Textarea
          id='media-options'
          value={options}
          onChange={(event) => setOptions(event.target.value)}
        />
        <Button
          disabled={
            !key.trim() || !model.trim() || !prompt.trim() || submit.isPending
          }
          onClick={() => submit.mutate()}
        >
          {submit.isPending ? t('Submitting...') : t('Create generation task')}
        </Button>
        <p className='text-muted-foreground text-sm'>
          {t(
            'If submission times out, check Task Logs before creating another task.'
          )}
        </p>
        {mode !== 'image_sync' && (
          <>
            <Label htmlFor='media-task'>{t('Task ID')}</Label>
            <div className='flex gap-2'>
              <Input
                id='media-task'
                value={taskId}
                onChange={(event) => setTaskId(event.target.value)}
              />
              <Button
                variant='outline'
                disabled={!key.trim() || !taskId.trim() || query.isPending}
                onClick={() => query.mutate()}
              >
                {t('Query task')}
              </Button>
            </div>
          </>
        )}
        {output && (
          <div className='space-y-2'>
            {mode === 'video' && output.status === 'completed' && (
              <Button
                variant='outline'
                disabled={download.isPending}
                onClick={() => download.mutate()}
              >
                {t('Download video')}
              </Button>
            )}
            <CopyButton value={JSON.stringify(output, null, 2)} size='sm'>
              {t('Copy response')}
            </CopyButton>
            <pre className='bg-muted max-h-96 overflow-auto rounded-lg p-4 text-xs'>
              {JSON.stringify(output, null, 2)}
            </pre>
          </div>
        )}
      </CardContent>
    </Card>
  )
}
