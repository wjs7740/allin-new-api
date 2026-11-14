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
import { useTranslation } from 'react-i18next'

import { CopyButton } from '@/components/copy-button'
import { SectionPageLayout } from '@/components/layout'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'

import { Assets } from './assets'
import { Connection } from './connection'
import { Workbench } from './workbench'

export function ZhiqiPlatform() {
  const { t } = useTranslation()
  const root = useAuthStore(
    (state) => state.auth.user?.role === ROLE.SUPER_ADMIN
  )
  const example = `curl ${window.location.origin}/v1/videos \\\n  -H 'Authorization: Bearer LOCAL_API_KEY' \\\n  -H 'Content-Type: application/json' \\\n  -d '{"model":"YOUR_VIDEO_MODEL","prompt":"A fox in a forest","duration":6}'`
  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('Media platform')}</SectionPageLayout.Title>
      <SectionPageLayout.Content>
        <Tabs defaultValue='workbench' className='space-y-5'>
          <TabsList className='max-w-full overflow-x-auto'>
            <TabsTrigger value='workbench'>{t('Workbench')}</TabsTrigger>
            <TabsTrigger value='assets'>{t('Asset library')}</TabsTrigger>
            <TabsTrigger value='docs'>{t('API guide')}</TabsTrigger>
            {root && (
              <TabsTrigger value='connection'>
                {t('Upstream connection')}
              </TabsTrigger>
            )}
          </TabsList>
          <TabsContent value='workbench'>
            <Workbench />
          </TabsContent>
          <TabsContent value='assets'>
            <Assets />
          </TabsContent>
          {root && (
            <TabsContent value='connection'>
              <Connection />
            </TabsContent>
          )}
          <TabsContent value='docs'>
            <Card>
              <CardHeader>
                <CardTitle>{t('API guide')}</CardTitle>
              </CardHeader>
              <CardContent className='space-y-4'>
                <p>
                  {t(
                    'Create a local API key and check GET /v1/models before making requests.'
                  )}
                </p>
                <pre className='bg-muted overflow-auto rounded-lg p-4 text-xs'>
                  {example}
                </pre>
                <CopyButton value={example} size='sm'>
                  {t('Copy example')}
                </CopyButton>
                <p>
                  {t(
                    'Video tasks use POST /v1/videos, GET /v1/videos/{task_id}, and GET /v1/videos/{task_id}/content. The standard gateway video status is completed on success.'
                  )}
                </p>
                <p>
                  {t(
                    'Asynchronous images use POST /v1/images/generations/async and GET /v1/images/generations/{task_id}. The successful image status is succeeded.'
                  )}
                </p>
                <p>
                  {t(
                    'Use public HTTP(S) URLs for media references. Add library_assets with local asset IDs to reuse materials you can access.'
                  )}
                </p>
                <p>
                  {t(
                    'Asset APIs use /api/aigc-assets/groups, /assets, and /uploads. Keep request_id unchanged when retrying the same asset operation and use the latest lock_version for changes.'
                  )}
                </p>
                <p>
                  {t(
                    'Text and synchronous images use the existing Chat, Responses, Messages, Images, Embeddings and Audio endpoints. Model capabilities and retail prices are configured by the administrator.'
                  )}
                </p>
              </CardContent>
            </Card>
          </TabsContent>
        </Tabs>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
