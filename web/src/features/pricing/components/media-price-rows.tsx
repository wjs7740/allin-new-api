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

import {
  formatTaskUsageUnitPrice,
  type DynamicPriceOptions,
} from '../lib/dynamic-price'
import { mediaPriceRows } from '../lib/media-price-rows'
import type { PricingModel } from '../types'

export function MediaPriceRows(props: {
  model: PricingModel
  options: DynamicPriceOptions
}) {
  const { t } = useTranslation()
  const rows = mediaPriceRows(props.model)
  if (!rows.length) return null
  return (
    <div className='col-span-full space-y-2'>
      <dl
        className='col-span-full space-y-2.5'
        aria-label={t('Prices by specification')}
      >
        {rows.map((row) => (
          <div
            key={row.label + row.unit}
            className='flex flex-wrap items-baseline justify-between gap-x-3 gap-y-1 text-xs'
          >
            <dt className='text-muted-foreground'>{row.label}</dt>
            <dd className='font-semibold tabular-nums'>
              {formatTaskUsageUnitPrice(row.price, props.options)}
              <span className='text-muted-foreground ml-1 font-normal'>
                / {t(row.unit)}
              </span>
            </dd>
          </div>
        ))}
      </dl>
      <p className='text-muted-foreground text-[11px] leading-4'>
        {t('Unspecified resolution uses the highest listed tier')}
      </p>
    </div>
  )
}
