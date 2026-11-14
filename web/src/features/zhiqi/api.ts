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
import { api } from '@/lib/api'
import { createServerError } from '@/lib/server-error-message'

type Envelope<T> = { success: boolean; message: string; data: T }
export type Asset = {
  id: string
  group_id?: string
  name: string
  description?: string
  scope: 'private' | 'public'
  kind?: string
  state: string
  lock_version: number
  can_manage: boolean
}
export type Catalog = {
  models: { id: string }[]
  pricing: unknown
  pricing_available: boolean
  plugin_source?: string
  plugin_version?: string
}

async function result<T>(request: Promise<{ data: Envelope<T> }>): Promise<T> {
  const response = (await request).data
  if (!response.success) throw createServerError(response)
  return response.data
}

export const getConfiguration = () =>
  result(
    api.get<Envelope<{ asset_channel_id: number }>>('/api/zhiqi/configuration')
  )
export const saveConfiguration = (asset_channel_id: number) =>
  result(
    api.put<Envelope<{ asset_channel_id: number }>>(
      '/api/zhiqi/configuration',
      { asset_channel_id }
    )
  )
export const previewCatalog = (input: {
  channel_id: number
  video_models: string[]
  image_models: string[]
}) => result(api.post<Envelope<Catalog>>('/api/zhiqi/catalog/preview', input))
export type AssetFilters = {
  p?: number
  keyword?: string
  scope?: string
  kind?: string
}
export const listAssets = (
  type: 'groups' | 'assets',
  group_id?: string,
  filters: AssetFilters = {}
) =>
  result(
    api.get<Envelope<{ items: Asset[]; total: number }>>(
      `/api/zhiqi-assets/${type}`,
      {
        params: {
          group_id,
          page_size: type === 'groups' ? 100 : 24,
          ...filters,
        },
      }
    )
  )
export const createAsset = (
  type: 'groups' | 'assets' | 'uploads',
  input: Record<string, unknown>
) => result(api.post<Envelope<Asset>>(`/api/zhiqi-assets/${type}`, input))
export const refreshAsset = (type: 'groups' | 'assets', id: string) =>
  result(
    api.get<Envelope<Asset>>(
      `/api/zhiqi-assets/${type}/${encodeURIComponent(id)}`,
      { params: { refresh: true } }
    )
  )
export const removeAsset = (type: 'groups' | 'assets', item: Asset) =>
  result(
    api.delete<Envelope<Asset>>(
      `/api/zhiqi-assets/${type}/${encodeURIComponent(item.id)}`,
      { params: { confirm: true }, data: { lock_version: item.lock_version } }
    )
  )
export const renameAsset = (
  type: 'groups' | 'assets',
  item: Asset,
  name: string
) =>
  result(
    api.put<Envelope<Asset>>(
      `/api/zhiqi-assets/${type}/${encodeURIComponent(item.id)}`,
      {
        name,
        description: item.description || '',
        lock_version: item.lock_version,
      }
    )
  )
export const previewAsset = (id: string) =>
  result(
    api.get<Envelope<{ url: string }>>(
      `/api/zhiqi-assets/assets/${encodeURIComponent(id)}/preview`
    )
  )

export const reconcileTask = (input: {
  task_id: string
  upstream_task_id: string
}) =>
  result(
    api.post<Envelope<{ task_id: string; status: string }>>(
      '/api/zhiqi/tasks/reconcile',
      input
    )
  )

export const completeUpload = (item: Asset, etag?: string) =>
  result(
    api.post<Envelope<Asset>>(
      `/api/zhiqi-assets/uploads/${encodeURIComponent(item.id)}/complete`,
      { etag, lock_version: item.lock_version }
    )
  )

export async function uploadAsset(
  groupId: string,
  name: string,
  kind: string,
  file: File,
  requestId: string
) {
  const authorization = await result(
    api.post<
      Envelope<{
        asset: Asset
        upload_url: string
        upload_token: string
        object_key: string
      }>
    >('/api/zhiqi-assets/uploads', {
      group_id: groupId,
      name,
      kind,
      mime_type: file.type,
      size_bytes: file.size,
      request_id: requestId,
    })
  )
  if (!authorization.upload_url || !authorization.asset) {
    throw new Error(
      'Upload already exists. Refresh the material and confirm the upload if the file was sent.'
    )
  }
  const target = new URL(authorization.upload_url)
  if (
    !['https:', 'http:'].includes(target.protocol) ||
    target.username ||
    target.password
  ) {
    throw new Error('Invalid upload URL')
  }
  const body = new FormData()
  body.set('token', authorization.upload_token)
  body.set('key', authorization.object_key)
  body.set('file', file)
  const uploaded = await fetch(target, {
    method: 'POST',
    body,
    credentials: 'omit',
    referrerPolicy: 'no-referrer',
    redirect: 'error',
  })
  if (!uploaded.ok) {
    throw new Error('Upload failed; refresh the asset before retrying')
  }
  const receipt = (await uploaded.json()) as { hash?: string }
  return completeUpload(authorization.asset, receipt.hash)
}
