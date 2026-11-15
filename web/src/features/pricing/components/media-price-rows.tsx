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

import { toIntlLocale } from '@/i18n/languages'
import { formatNumber } from '@/lib/format'

import {
  formatTaskUsageUnitPrice,
  type DynamicPriceOptions,
} from '../lib/dynamic-price'
import { mediaPriceRows } from '../lib/media-price-rows'
import {
  OFFICIAL_PRICE_DATE,
  officialPriceDiscount,
  officialPriceReference,
} from '../lib/official-prices'
import type { PricingModel } from '../types'

export function MediaPriceRows(props: {
  model: PricingModel
  options: DynamicPriceOptions
}) {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const rows = mediaPriceRows(props.model)
  if (!rows.length) return null
  const reference = rows
    .map((row) => officialPriceReference(props.model.model_name, row.label))
    .find(Boolean)
  return (
    <div className='col-span-full space-y-2'>
      <dl
        className='col-span-full space-y-2.5'
        aria-label={t('Prices by specification')}
      >
        {rows.map((row) => {
          const official = officialPriceReference(
            props.model.model_name,
            row.label
          )
          const ratio = official
            ? officialPriceDiscount(
                row.price,
                official,
                props.options.usdExchangeRate ?? 1,
                props.options.groupRatioMultiplier
              )
            : null
          let comparison = ''
          if (ratio !== null) {
            if (ratio > 1 + 1e-8) comparison = t('Above official reference')
            else if (ratio >= 0.995) comparison = t('At official reference')
            else {
              comparison = t('Official reference {{discount}}/10', {
                discount: formatNumber(Math.round(ratio * 100) / 10, locale),
              })
            }
          }
          return (
            <div
              key={row.label + row.unit}
              className='flex flex-wrap items-baseline justify-between gap-x-3 gap-y-1 text-xs'
            >
              <dt className='text-muted-foreground'>{t(row.label)}</dt>
              <dd className='font-semibold tabular-nums'>
                {formatTaskUsageUnitPrice(row.price, props.options)}
                <span className='text-muted-foreground ml-1 font-normal'>
                  / {t(row.unit)}
                </span>
                {official && comparison && (
                  <a
                    className='text-primary mt-1 block text-right text-[10px] font-normal hover:underline'
                    href={official.source}
                    target='_blank'
                    rel='noopener noreferrer'
                    title={`${t(official.basis)} · ${OFFICIAL_PRICE_DATE}`}
                  >
                    {comparison}
                  </a>
                )}
              </dd>
            </div>
          )
        })}
      </dl>
      <p className='text-muted-foreground text-[11px] leading-4'>
        {t('Unspecified resolution uses the highest listed tier')}
      </p>
      <p className='text-muted-foreground text-[10px] leading-4'>
        {reference ? (
          <a href='/docs/pricing' className='hover:text-primary'>
            {t(reference.basis)} · {OFFICIAL_PRICE_DATE}
          </a>
        ) : (
          t('No verified official price for this specification')
        )}
      </p>
    </div>
  )
}
