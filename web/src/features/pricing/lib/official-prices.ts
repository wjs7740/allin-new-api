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
// Public online prices checked on 2026-11-15. These are comparison references,
// never an input to the billing expression. Promotional packages are excluded.
export const OFFICIAL_PRICE_DATE = '2026-11-15'
const ark = 'https://docs.volcengine.com/docs/ark/model-pricing?lang=zh'
const openai =
  'https://developers.openai.com/api/docs/guides/image-generation#calculating-costs'

export type OfficialReference = {
  price: number
  currency: 'CNY' | 'USD'
  source: string
  basis: string
}

export function officialPriceReference(
  model: string,
  label: string
): OfficialReference | null {
  if (model.startsWith('gpt-image-')) {
    const prices = model.startsWith('gpt-image-2.6')
      ? { '1K': 0.02469, '2K': 0.05001, '4K': 0.04674 }
      : { '1K': 0.00588, '2K': 0.01191, '4K': 0.01113 }
    const price = prices[label as keyof typeof prices]
    if (!price) return null
    return {
      price,
      currency: 'USD',
      source: openai,
      basis: 'Low quality: 1K/2K square, 4K 3840×2160; output only',
    }
  }
  if (model === 'H3' && label === '720p') {
    return {
      price: 0.08,
      currency: 'USD',
      source: 'https://platform.minimax.io/docs/pricing/overview',
      basis: 'H3 768P output; channel parameter 720p',
    }
  }
  if (model === 'seedance-2.0-fast-token') {
    if (!['reference-video', 'text-or-image'].includes(label)) return null
    return {
      price: label === 'reference-video' ? 22 : 37,
      currency: 'CNY',
      source: ark,
      basis:
        'Online price per million tokens; reference-video tier shown separately',
    }
  }
  if (model === 'seedance-2.0-mini-token' && ['480p', '720p'].includes(label)) {
    return {
      price: 23,
      currency: 'CNY',
      source: ark,
      basis: 'No input video: ¥23/M tokens; with video: ¥14/M tokens',
    }
  }
  const prices: Record<string, Record<string, number>> = {
    'seedance-2.0': { '480p': 0.46, '720p': 0.99 },
    'seedance-2.0-fast': { '480p': 0.37, '720p': 0.8 },
    'seedance-2.0-mini': { '480p': 0.23, '720p': 0.5 },
    'seedance-2.5': { '480p': 0.67, '720p': 1.51, '1080p': 3.74 },
    'seedance-2.5-ref': { '480p': 0.726, '720p': 1.632, '1080p': 4.024 },
  }
  const price = prices[model]?.[label]
  if (!price) return null
  return {
    price,
    currency: 'CNY',
    source: ark,
    basis: model.endsWith('-ref')
      ? 'Official example: 16:9, 5s output, 2–4s input video'
      : 'Official example: 16:9, 5s, no input video; token billing',
  }
}

export function officialPriceDiscount(
  saleUSD: number,
  reference: OfficialReference,
  exchangeRate: number,
  groupRatio = 1
): number | null {
  if (
    ![saleUSD, exchangeRate, groupRatio].every(Number.isFinite) ||
    saleUSD < 0 ||
    exchangeRate <= 0 ||
    groupRatio < 0
  ) {
    return null
  }
  const officialCNY =
    reference.price * (reference.currency === 'USD' ? exchangeRate : 1)
  return (saleUSD * exchangeRate * groupRatio) / officialCNY
}
