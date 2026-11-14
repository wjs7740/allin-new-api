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
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { getCoreRowModel, useReactTable } from '@tanstack/react-table'
import { useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { CopyButton } from '@/components/copy-button'
import { DataTablePagination } from '@/components/data-table'
import { EmptyState } from '@/components/empty-state'
import { LoadingState } from '@/components/loading-state'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'

import {
  createAsset,
  completeUpload,
  listAssets,
  previewAsset,
  refreshAsset,
  removeAsset,
  renameAsset,
  uploadAsset,
  type Asset,
} from './api'

export function Assets() {
  const { t } = useTranslation()
  const client = useQueryClient()
  const [group, setGroup] = useState('')
  const [name, setName] = useState('')
  const [groupName, setGroupName] = useState('')
  const [scope, setScope] = useState('private')
  const admin = useAuthStore(
    (state) => (state.auth.user?.role ?? 0) >= ROLE.ADMIN
  )
  const groupRequest = useRef({ signature: '', key: crypto.randomUUID() })
  const assetRequest = useRef({ signature: '', key: crypto.randomUUID() })
  const [pagination, setPagination] = useState({ pageIndex: 0, pageSize: 24 })
  const [groupPagination, setGroupPagination] = useState({
    pageIndex: 0,
    pageSize: 100,
  })
  const [search, setSearch] = useState('')
  const [keyword, setKeyword] = useState('')
  const [filterScope, setFilterScope] = useState('')
  const [renameType, setRenameType] = useState<'groups' | 'assets'>('assets')
  const [kind, setKind] = useState('image')
  const [url, setUrl] = useState('')
  const [file, setFile] = useState<File | null>(null)
  const [deleting, setDeleting] = useState<{
    type: 'groups' | 'assets'
    item: Asset
  } | null>(null)
  const [renaming, setRenaming] = useState<Asset | null>(null)
  const [newName, setNewName] = useState('')
  const [preview, setPreview] = useState<{ url: string; kind?: string } | null>(
    null
  )
  const groups = useQuery({
    queryKey: ['zhiqi-assets', 'groups', groupPagination],
    queryFn: () =>
      listAssets('groups', undefined, { p: groupPagination.pageIndex + 1 }),
  })
  const assets = useQuery({
    queryKey: [
      'zhiqi-assets',
      'assets',
      group,
      pagination,
      keyword,
      filterScope,
    ],
    queryFn: () =>
      listAssets('assets', group || undefined, {
        p: pagination.pageIndex + 1,
        keyword,
        scope: filterScope,
      }),
  })
  const table = useReactTable({
    data: assets.data?.items ?? [],
    columns: [],
    rowCount: assets.data?.total ?? 0,
    manualPagination: true,
    state: { pagination },
    onPaginationChange: setPagination,
    getCoreRowModel: getCoreRowModel(),
  })
  const groupTable = useReactTable({
    data: groups.data?.items ?? [],
    columns: [],
    rowCount: groups.data?.total ?? 0,
    manualPagination: true,
    state: { pagination: groupPagination },
    onPaginationChange: setGroupPagination,
    getCoreRowModel: getCoreRowModel(),
  })
  const reload = () => {
    void client.invalidateQueries({ queryKey: ['zhiqi-assets'] })
  }
  const createGroup = useMutation({
    mutationFn: () => {
      const signature = JSON.stringify({ name: groupName, scope })
      if (groupRequest.current.signature !== signature) {
        groupRequest.current = { signature, key: crypto.randomUUID() }
      }
      return createAsset('groups', {
        name: groupName,
        scope,
        request_id: groupRequest.current.key,
      })
    },
    onSuccess: (item) => {
      setGroup(item.id)
      setGroupName('')
      groupRequest.current = { signature: '', key: crypto.randomUUID() }
      reload()
    },
  })
  const create = useMutation({
    mutationFn: () => {
      const signature = JSON.stringify({
        group,
        name,
        kind,
        url,
        file: file
          ? [file.name, file.type, file.size, file.lastModified]
          : null,
      })
      if (assetRequest.current.signature !== signature) {
        assetRequest.current = { signature, key: crypto.randomUUID() }
      }
      return file
        ? uploadAsset(group, name, kind, file, assetRequest.current.key)
        : createAsset('assets', {
            group_id: group,
            name,
            kind,
            url,
            request_id: assetRequest.current.key,
          })
    },
    onSuccess: () => {
      assetRequest.current = { signature: '', key: crypto.randomUUID() }
      reload()
    },
    onSettled: reload,
  })
  const refresh = useMutation({
    mutationFn: (item: { type: 'groups' | 'assets'; id: string }) =>
      refreshAsset(item.type, item.id),
    onSuccess: reload,
  })
  const remove = useMutation({
    mutationFn: (item: { type: 'groups' | 'assets'; item: Asset }) =>
      removeAsset(item.type, item.item),
    onSuccess: () => {
      setDeleting(null)
      reload()
    },
  })
  const rename = useMutation({
    mutationFn: () => {
      if (!renaming) throw new Error('Select a material')
      return renameAsset(renameType, renaming, newName)
    },
    onSuccess: () => {
      setRenaming(null)
      reload()
    },
  })
  const show = useMutation({
    mutationFn: async (item: Asset) => ({
      ...(await previewAsset(item.id)),
      kind: item.kind,
    }),
    onSuccess: setPreview,
  })
  const complete = useMutation({
    mutationFn: (item: Asset) => completeUpload(item),
    onSettled: reload,
  })
  const selected = groups.data?.items.find((item) => item.id === group)
  return (
    <div className='space-y-6'>
      <Card>
        <CardHeader>
          <CardTitle>{t('Asset groups')}</CardTitle>
        </CardHeader>
        <CardContent className='space-y-3'>
          {admin && (
            <>
              <Label htmlFor='asset-scope'>{t('Visibility')}</Label>
              <NativeSelect
                id='asset-scope'
                value={scope}
                onChange={(event) => setScope(event.target.value)}
              >
                <NativeSelectOption value='private'>
                  {t('Private')}
                </NativeSelectOption>
                <NativeSelectOption value='public'>
                  {t('Public')}
                </NativeSelectOption>
              </NativeSelect>
            </>
          )}
          <Label htmlFor='asset-group-name'>{t('New material group')}</Label>
          <div className='flex gap-2'>
            <Input
              id='asset-group-name'
              value={groupName}
              maxLength={64}
              onChange={(event) => setGroupName(event.target.value)}
            />
            <Button
              disabled={!groupName.trim() || createGroup.isPending}
              onClick={() => createGroup.mutate()}
            >
              {t('Create')}
            </Button>
          </div>
          <Label htmlFor='asset-group'>{t('Material group')}</Label>
          <NativeSelect
            id='asset-group'
            value={group}
            onChange={(event) => {
              setGroup(event.target.value)
              setPagination({ pageIndex: 0, pageSize: 24 })
            }}
          >
            <NativeSelectOption value=''>
              {t('All material groups')}
            </NativeSelectOption>
            {groups.data?.items.map((item) => (
              <NativeSelectOption key={item.id} value={item.id}>
                {item.name} · {item.scope} · {item.state}
              </NativeSelectOption>
            ))}
          </NativeSelect>
          {(groups.data?.total ?? 0) > 100 && (
            <DataTablePagination table={groupTable} compact />
          )}
          {selected && (
            <div className='flex flex-wrap gap-2'>
              <CopyButton value={selected.id} size='sm'>
                {t('Copy group ID')}
              </CopyButton>
              <Button
                variant='outline'
                disabled={refresh.isPending}
                onClick={() =>
                  refresh.mutate({ type: 'groups', id: selected.id })
                }
              >
                {t('Refresh status')}
              </Button>
              {selected.can_manage && (
                <Button
                  variant='outline'
                  onClick={() => {
                    setRenameType('groups')
                    setRenaming(selected)
                    setNewName(selected.name)
                  }}
                >
                  {t('Rename group')}
                </Button>
              )}
              {selected.can_manage && (
                <Button
                  variant='outline'
                  onClick={() =>
                    setDeleting({ type: 'groups', item: selected })
                  }
                >
                  {t('Delete group')}
                </Button>
              )}
            </div>
          )}
        </CardContent>
      </Card>
      <Card>
        <CardHeader>
          <CardTitle>{t('Add material')}</CardTitle>
        </CardHeader>
        <CardContent className='grid gap-3 md:grid-cols-2'>
          <div className='space-y-2'>
            <Label htmlFor='asset-name'>{t('Name')}</Label>
            <Input
              id='asset-name'
              value={name}
              maxLength={64}
              onChange={(event) => setName(event.target.value)}
            />
          </div>
          <div className='space-y-2'>
            <Label htmlFor='asset-kind'>{t('Material type')}</Label>
            <NativeSelect
              id='asset-kind'
              value={kind}
              onChange={(event) => setKind(event.target.value)}
            >
              {['image', 'video', 'audio'].map((value) => (
                <NativeSelectOption key={value} value={value}>
                  {value}
                </NativeSelectOption>
              ))}
            </NativeSelect>
          </div>
          <div className='space-y-2'>
            <Label htmlFor='asset-url'>{t('Material URL')}</Label>
            <Input
              id='asset-url'
              type='url'
              value={url}
              disabled={!!file}
              onChange={(event) => setUrl(event.target.value)}
            />
          </div>
          <div className='space-y-2'>
            <Label htmlFor='asset-file'>{t('Local file')}</Label>
            <Input
              id='asset-file'
              type='file'
              onChange={(event) => setFile(event.target.files?.[0] ?? null)}
            />
          </div>
          <Button
            disabled={
              !selected?.can_manage ||
              selected.state !== 'active' ||
              !name.trim() ||
              (!file && !url.trim()) ||
              create.isPending
            }
            onClick={() => create.mutate()}
          >
            {create.isPending ? t('Uploading...') : t('Add material')}
          </Button>
        </CardContent>
      </Card>
      <form
        className='flex flex-wrap items-end gap-3'
        onSubmit={(event) => {
          event.preventDefault()
          setKeyword(search)
          setPagination({ pageIndex: 0, pageSize: 24 })
        }}
      >
        <div className='space-y-2'>
          <Label htmlFor='asset-search'>{t('Search materials')}</Label>
          <Input
            id='asset-search'
            value={search}
            onChange={(event) => setSearch(event.target.value)}
          />
        </div>
        <div className='space-y-2'>
          <Label htmlFor='asset-filter-scope'>{t('Material visibility')}</Label>
          <NativeSelect
            id='asset-filter-scope'
            value={filterScope}
            onChange={(event) => {
              setFilterScope(event.target.value)
              setPagination({ pageIndex: 0, pageSize: 24 })
            }}
          >
            <NativeSelectOption value=''>{t('All')}</NativeSelectOption>
            <NativeSelectOption value='private'>
              {t('Private')}
            </NativeSelectOption>
            <NativeSelectOption value='public'>
              {t('Public')}
            </NativeSelectOption>
          </NativeSelect>
        </div>
        <Button type='submit' variant='outline'>
          {t('Search')}
        </Button>
      </form>
      {assets.isPending && <LoadingState />}
      {assets.isError && (
        <Button variant='outline' onClick={() => void assets.refetch()}>
          {t('Retry')}
        </Button>
      )}
      {assets.data?.items.length === 0 && (
        <EmptyState
          title={t('No materials yet')}
          description={t('Create a group, then add a URL or upload a file.')}
        />
      )}
      <div className='grid gap-4 md:grid-cols-2 xl:grid-cols-3'>
        {assets.data?.items.map((item) => (
          <Card key={item.id}>
            <CardHeader>
              <CardTitle className='break-words'>{item.name}</CardTitle>
            </CardHeader>
            <CardContent className='space-y-3'>
              <p className='text-muted-foreground text-sm'>
                {item.kind} · {item.scope} · {item.state}
              </p>
              <div className='flex flex-wrap gap-2'>
                <CopyButton value={item.id} size='sm'>
                  {t('Copy asset ID')}
                </CopyButton>
                <Button
                  variant='outline'
                  size='sm'
                  disabled={refresh.isPending}
                  onClick={() =>
                    refresh.mutate({ type: 'assets', id: item.id })
                  }
                >
                  {t('Refresh')}
                </Button>
                <Button
                  variant='outline'
                  size='sm'
                  disabled={item.state !== 'active' || show.isPending}
                  onClick={() => show.mutate(item)}
                >
                  {t('Preview')}
                </Button>
              </div>
              {item.can_manage && (
                <div className='flex gap-2'>
                  <Button
                    variant='ghost'
                    size='sm'
                    onClick={() => {
                      setRenameType('assets')
                      setRenaming(item)
                      setNewName(item.name)
                    }}
                  >
                    {t('Rename')}
                  </Button>
                  {item.state === 'uploading' && (
                    <Button
                      variant='ghost'
                      size='sm'
                      disabled={complete.isPending}
                      onClick={() => complete.mutate(item)}
                    >
                      {t('Confirm uploaded file')}
                    </Button>
                  )}
                  <Button
                    variant='ghost'
                    size='sm'
                    onClick={() => setDeleting({ type: 'assets', item })}
                  >
                    {t('Delete')}
                  </Button>
                </div>
              )}
            </CardContent>
          </Card>
        ))}
      </div>
      {(assets.data?.total ?? 0) > 0 && (
        <DataTablePagination table={table} compact />
      )}
      {preview && (
        <Card>
          <CardContent className='space-y-3 pt-4'>
            <Button variant='outline' onClick={() => setPreview(null)}>
              {t('Close preview')}
            </Button>
            {preview.kind === 'image' && (
              <img
                src={preview.url}
                alt={t('Material preview')}
                className='max-h-96 rounded-lg object-contain'
                referrerPolicy='no-referrer'
              />
            )}
            {preview.kind === 'video' && (
              <video src={preview.url} controls className='max-h-96' />
            )}
            {preview.kind === 'audio' && <audio src={preview.url} controls />}
          </CardContent>
        </Card>
      )}
      {renaming && (
        <Card>
          <CardContent className='space-y-2 pt-4'>
            <Label htmlFor='asset-rename'>{t('New name')}</Label>
            <Input
              id='asset-rename'
              value={newName}
              onChange={(event) => setNewName(event.target.value)}
            />
            <Button
              disabled={!newName.trim() || rename.isPending}
              onClick={() => rename.mutate()}
            >
              {t('Save')}
            </Button>
            <Button variant='ghost' onClick={() => setRenaming(null)}>
              {t('Cancel')}
            </Button>
          </CardContent>
        </Card>
      )}
      <ConfirmDialog
        open={!!deleting}
        onOpenChange={(open) => {
          if (!open) setDeleting(null)
        }}
        title={t('Delete material')}
        desc={t('Deleting a group also deletes its materials.')}
        handleConfirm={() => {
          if (deleting) remove.mutate(deleting)
        }}
        isLoading={remove.isPending}
        destructive
      />
    </div>
  )
}
