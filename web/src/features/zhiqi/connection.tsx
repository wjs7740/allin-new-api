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
import { useMutation, useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { CopyButton } from '@/components/copy-button'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import {
  activateTaskPlugin,
  uploadTaskPlugin,
} from '@/features/task-plugins/api'

import {
  getConfiguration,
  previewCatalog,
  saveConfiguration,
  reconcileTask,
} from './api'

export function Connection() {
  const { t } = useTranslation()
  const [channel, setChannel] = useState('')
  const [videos, setVideos] = useState('')
  const [images, setImages] = useState('')
  const [localTask, setLocalTask] = useState('')
  const [upstreamTask, setUpstreamTask] = useState('')
  const reconcile = useMutation({
    mutationFn: reconcileTask,
    onSuccess: () => toast.success(t('Task receipt linked')),
  })
  const config = useQuery({
    queryKey: ['zhiqi-config'],
    queryFn: getConfiguration,
  })
  const catalog = useMutation({ mutationFn: previewCatalog })
  const install = useMutation({
    mutationFn: async (source: string) => {
      const installed = await uploadTaskPlugin(source, 'Zhiqi catalog')
      return activateTaskPlugin('zhiqi', installed.meta.version)
    },
    onSuccess: () => toast.success(t('Plugin installed')),
  })
  const save = useMutation({
    mutationFn: saveConfiguration,
    onSuccess: () => {
      void config.refetch()
      toast.success(t('Saved'))
    },
  })
  const channelId = Number(channel || config.data?.asset_channel_id || 0)
  const names = (text: string) =>
    text
      .split(/[\n,]+/)
      .map((name) => name.trim())
      .filter(Boolean)
  return (
    <div className='space-y-6'>
      <Card>
        <CardHeader>
          <CardTitle>{t('Zhiqi connection')}</CardTitle>
        </CardHeader>
        <CardContent className='space-y-4'>
          <p className='text-muted-foreground text-sm'>
            {t(
              'Create a Plugin channel, select Zhiqi, and enter your upstream API key in Channels. Use one key per channel.'
            )}
          </p>
          <Label htmlFor='zhiqi-channel'>{t('Channel ID')}</Label>
          <Input
            id='zhiqi-channel'
            type='number'
            min='1'
            value={channel || String(config.data?.asset_channel_id || '')}
            onChange={(event) => setChannel(event.target.value)}
          />
          <div className='grid gap-4 md:grid-cols-2'>
            <div className='space-y-2'>
              <Label htmlFor='zhiqi-videos'>
                {t('Video models, one per line')}
              </Label>
              <Textarea
                id='zhiqi-videos'
                value={videos}
                onChange={(event) => setVideos(event.target.value)}
              />
            </div>
            <div className='space-y-2'>
              <Label htmlFor='zhiqi-images'>
                {t('Asynchronous image models, one per line')}
              </Label>
              <Textarea
                id='zhiqi-images'
                value={images}
                onChange={(event) => setImages(event.target.value)}
              />
            </div>
          </div>
          <div className='flex flex-wrap gap-2'>
            <Button
              disabled={channelId < 1 || catalog.isPending}
              onClick={() =>
                catalog.mutate({
                  channel_id: channelId,
                  video_models: names(videos),
                  image_models: names(images),
                })
              }
            >
              {catalog.isPending
                ? t('Loading...')
                : t('Preview upstream catalog')}
            </Button>
            <Button
              variant='outline'
              disabled={channelId < 1 || save.isPending}
              onClick={() => save.mutate(channelId)}
            >
              {t('Use this channel for assets')}
            </Button>
          </div>
          <p className='text-muted-foreground text-sm'>
            {t(
              'Select only models with confirmed endpoint support. Set local retail prices before enabling customer traffic.'
            )}
          </p>
        </CardContent>
      </Card>
      {catalog.data && (
        <Card>
          <CardHeader>
            <CardTitle>{t('Upstream catalog')}</CardTitle>
          </CardHeader>
          <CardContent className='space-y-4'>
            <pre className='bg-muted max-h-64 overflow-auto rounded-lg p-4 text-xs'>
              {catalog.data.models.map((item) => item.id).join('\n')}
            </pre>
            <CopyButton
              value={catalog.data.models.map((item) => item.id).join('\n')}
              size='sm'
            >
              {t('Copy models')}
            </CopyButton>
            <p className='text-sm'>
              {catalog.data.pricing_available
                ? t(
                    'Upstream costs are available for review; local prices are unchanged.'
                  )
                : t('This key did not return a pricing catalog.')}
            </p>
            {catalog.data.pricing_available && (
              <pre className='bg-muted max-h-64 overflow-auto rounded-lg p-4 text-xs'>
                {JSON.stringify(catalog.data.pricing, null, 2)}
              </pre>
            )}
            {catalog.data.plugin_source && (
              <div className='flex gap-2'>
                <Button
                  disabled={install.isPending}
                  onClick={() => {
                    if (catalog.data?.plugin_source) {
                      install.mutate(catalog.data.plugin_source)
                    }
                  }}
                >
                  {t('Install selected model catalog')}
                </Button>
                <CopyButton value={catalog.data.plugin_source} size='sm'>
                  {t('Copy plugin source')}
                </CopyButton>
              </div>
            )}
          </CardContent>
        </Card>
      )}
      <Card>
        <CardHeader>
          <CardTitle>{t('Reconcile uncertain submission')}</CardTitle>
        </CardHeader>
        <CardContent className='space-y-3'>
          <p className='text-muted-foreground text-sm'>
            {t(
              'After verifying the upstream receipt, link it to the existing local task. This does not generate again.'
            )}
          </p>
          <Label htmlFor='reconcile-local'>{t('Local task ID')}</Label>
          <Input
            id='reconcile-local'
            value={localTask}
            onChange={(event) => setLocalTask(event.target.value)}
          />
          <Label htmlFor='reconcile-upstream'>
            {t('Verified upstream task ID')}
          </Label>
          <Input
            id='reconcile-upstream'
            value={upstreamTask}
            onChange={(event) => setUpstreamTask(event.target.value)}
          />
          <Button
            disabled={
              !localTask.trim() || !upstreamTask.trim() || reconcile.isPending
            }
            onClick={() =>
              reconcile.mutate({
                task_id: localTask.trim(),
                upstream_task_id: upstreamTask.trim(),
              })
            }
          >
            {t('Link receipt and resume polling')}
          </Button>
        </CardContent>
      </Card>
    </div>
  )
}
