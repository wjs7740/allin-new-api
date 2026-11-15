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
import { Link } from '@tanstack/react-router'
import { ArrowLeft, ArrowRight, BookOpen, Search } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { CodeBlock } from '@/components/ai-elements/code-block'
import { CopyButton } from '@/components/copy-button'
import { PublicLayout } from '@/components/layout'
import { Input } from '@/components/ui/input'
import { Markdown } from '@/components/ui/markdown'
import { cn } from '@/lib/utils'

import pages from './content.json'

export function Documentation(props: { slug?: string }) {
  const { t, i18n } = useTranslation()
  const [search, setSearch] = useState('')
  const lang = i18n.resolvedLanguage?.startsWith('zh') ? 'zh' : 'en'
  const slug = props.slug?.replace(/\/+$/, '') || 'quickstart'
  const index = pages.findIndex((item) => item.slug === slug)
  const page = pages[index]
  const groups = [...new Set(pages.map((item) => item.group))]
  const query = search.trim().toLowerCase()
  const filtered = pages.filter((item) =>
    [
      item.title[lang],
      item.description[lang],
      ...item.sections.map(
        (section) =>
          `${section.title[lang]} ${section.body[lang]} ${section.code}`
      ),
    ]
      .join(' ')
      .toLowerCase()
      .includes(query)
  )
  const origin = window.location.origin

  return (
    <PublicLayout showMainContainer={false}>
      <div className='mx-auto max-w-[1440px] px-4 pt-20 lg:px-8'>
        <div className='grid gap-8 lg:grid-cols-[224px_minmax(0,1fr)] xl:grid-cols-[224px_minmax(0,1fr)_180px]'>
          <aside className='lg:sticky lg:top-20 lg:h-[calc(100vh-6rem)] lg:overflow-y-auto lg:border-r lg:pr-5'>
            <div className='mb-5 flex items-center gap-2 text-sm font-semibold'>
              <BookOpen className='text-primary size-4' />
              {t('Developer documentation')}
            </div>
            <div className='relative mb-5'>
              <Search className='text-muted-foreground pointer-events-none absolute top-2.5 left-3 size-4' />
              <Input
                className='pl-9'
                value={search}
                onChange={(event) => setSearch(event.target.value)}
                placeholder={t('Search documentation')}
                aria-label={t('Search documentation')}
              />
            </div>
            <nav
              aria-label={t('Documentation navigation')}
              className='max-h-60 space-y-5 overflow-y-auto pb-4 lg:max-h-none'
            >
              {groups.map((group) => {
                const items = filtered.filter((item) => item.group === group)
                if (!items.length) return null
                return (
                  <div key={group}>
                    <p className='text-muted-foreground mb-2 px-3 text-xs font-medium'>
                      {t(group)}
                    </p>
                    {items.map((item) => (
                      <Link
                        key={item.slug}
                        to='/docs/$'
                        params={{ _splat: item.slug }}
                        aria-current={slug === item.slug ? 'page' : undefined}
                        onClick={() => window.scrollTo(0, 0)}
                        className={cn(
                          'block rounded-md px-3 py-2 text-sm transition-colors hover:bg-muted focus-visible:outline-primary',
                          slug === item.slug &&
                            'bg-primary/8 text-primary font-semibold'
                        )}
                      >
                        {item.title[lang]}
                      </Link>
                    ))}
                  </div>
                )
              })}
              {!filtered.length && (
                <p className='text-muted-foreground px-3 text-sm'>
                  {t('No matching documentation')}
                </p>
              )}
            </nav>
          </aside>
          <main className='min-w-0 pb-16'>
            {!page ? (
              <div className='py-12'>
                <h1 className='text-3xl font-semibold'>
                  {t('Document not found')}
                </h1>
                <Link to='/docs' className='text-primary mt-4 inline-block'>
                  {t('Back to quickstart')}
                </Link>
              </div>
            ) : (
              <>
                <header className='border-b pb-7'>
                  <p className='text-primary mb-3 text-sm'>{t(page.group)}</p>
                  <h1 className='text-3xl font-semibold tracking-tight sm:text-4xl'>
                    {page.title[lang]}
                  </h1>
                  <p className='text-muted-foreground mt-4 text-base leading-7'>
                    {page.description[lang]}
                  </p>
                </header>
                {page.slug === 'quickstart' && (
                  <div className='bg-muted/45 my-7 flex flex-wrap items-center gap-3 rounded-lg border px-4 py-3'>
                    <span className='text-muted-foreground text-xs'>
                      Base URL
                    </span>
                    <code className='min-w-0 flex-1 text-sm break-all'>
                      {origin}/v1
                    </code>
                    <CopyButton
                      value={`${origin}/v1`}
                      aria-label={t('Copy Base URL')}
                    />
                  </div>
                )}
                {page.sections.map((section) => (
                  <section
                    key={section.id}
                    id={section.id}
                    className='scroll-mt-24 pt-9'
                  >
                    <h2 className='mb-4 text-xl font-semibold tracking-tight'>
                      <a href={`#${section.id}`}>{section.title[lang]}</a>
                    </h2>
                    <Markdown className='text-sm leading-7 [&_table]:block [&_table]:overflow-x-auto'>
                      {section.body[lang]}
                    </Markdown>
                    {section.code && (
                      <div className='mt-5 min-w-0 overflow-hidden rounded-lg border'>
                        <CodeBlock
                          code={section.code.replaceAll('{{origin}}', origin)}
                          language={section.language}
                          showToolbar
                          enableCollapse={false}
                        >
                          <CopyButton
                            value={section.code.replaceAll(
                              '{{origin}}',
                              origin
                            )}
                            className='size-8'
                          />
                        </CodeBlock>
                      </div>
                    )}
                  </section>
                ))}
                <footer className='mt-12 flex flex-wrap justify-between gap-5 border-t pt-6 text-sm'>
                  {pages[index - 1] ? (
                    <Link
                      to='/docs/$'
                      params={{ _splat: pages[index - 1].slug }}
                      onClick={() => window.scrollTo(0, 0)}
                      className='text-muted-foreground hover:text-primary flex items-center gap-2'
                    >
                      <ArrowLeft className='size-4' />
                      {pages[index - 1].title[lang]}
                    </Link>
                  ) : (
                    <span />
                  )}
                  {pages[index + 1] && (
                    <Link
                      to='/docs/$'
                      params={{ _splat: pages[index + 1].slug }}
                      onClick={() => window.scrollTo(0, 0)}
                      className='text-primary flex items-center gap-2'
                    >
                      {pages[index + 1].title[lang]}
                      <ArrowRight className='size-4' />
                    </Link>
                  )}
                </footer>
              </>
            )}
          </main>
          {page && (
            <aside className='hidden xl:block'>
              <nav
                className='sticky top-24 border-l pl-5 text-xs'
                aria-label={t('On this page')}
              >
                <p className='mb-4 font-semibold'>{t('On this page')}</p>
                {page.sections.map((section) => (
                  <a
                    className='text-muted-foreground hover:text-primary mb-3 block leading-5'
                    key={section.id}
                    href={`#${section.id}`}
                  >
                    {section.title[lang]}
                  </a>
                ))}
                <a
                  href='/pricing'
                  className='text-primary mt-6 block border-t pt-4'
                >
                  {t('Live model prices')}
                </a>
              </nav>
            </aside>
          )}
        </div>
      </div>
    </PublicLayout>
  )
}
