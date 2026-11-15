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
import { describe, expect, it } from 'vitest'

import { mediaPriceRows } from '../lib/media-price-rows'
import {
  officialPriceDiscount,
  officialPriceReference,
} from '../lib/official-prices'
import type { PricingModel } from '../types'

describe('official price comparison', () => {
  it('compares like currencies and applies the selected group exactly once', () => {
    const reference = officialPriceReference('gpt-image-2.6', '1K')
    if (!reference) throw new Error('Missing official reference')
    expect(officialPriceDiscount(0.04 / 6.8, reference, 6.8)).toBeCloseTo(
      0.238248
    )
    expect(officialPriceDiscount(0.04 / 6.8, reference, 6.8, 2)).toBeCloseTo(
      0.476497
    )
    expect(
      officialPriceDiscount(0.04 / 6.8, reference, 6.8, 10)
    ).toBeGreaterThan(1)
    expect(officialPriceDiscount(1, reference, 0)).toBeNull()
  })
  it('does not fabricate same-spec references for Muse, GIF or unknown video tiers', () => {
    expect(officialPriceReference('muse-video', '720p')).toBeNull()
    expect(officialPriceReference('gpt-image-2.6', 'GIF')).toBeNull()
    expect(officialPriceReference('seedance-2.0-fast-token', '480p')).toBeNull()
  })
  it('uses the correct generation and reference-video official rates', () => {
    expect(officialPriceReference('gpt-image-2.5', '1K')?.price).toBe(0.00588)
    expect(officialPriceReference('gpt-image-2.6', '1K')?.price).toBe(0.02469)
    expect(
      officialPriceReference('seedance-2.0-fast-token', 'reference-video')
        ?.price
    ).toBe(22)
    expect(
      officialPriceReference('seedance-2.0-fast-token', 'text-or-image')?.price
    ).toBe(37)
  })
  it('reads reference-video prices from the executed task expression', () => {
    const model = {
      id: 1,
      quota_type: 0,
      model_ratio: 1,
      completion_ratio: 1,
      enable_groups: ['default'],
      model_name: 'seedance-2.0-fast-token',
      supported_endpoint_types: ['openai-video'],
      billing_usage_schema: {
        tokens: { type: 'number', unit: 'token' },
        has_video: { type: 'boolean' },
      },
      billing_expr:
        'u("has_video") == true ? tier("reference-video", u("tokens") * 3.073529411764706 / 1000000) : tier("text-or-image", u("tokens") * 4.220588235294118 / 1000000)',
    } as PricingModel
    const rows = mediaPriceRows(model)
    expect(rows).toHaveLength(2)
    expect(rows[0].price * 6.8).toBeCloseTo(20.9)
    expect(rows[1].price * 6.8).toBeCloseTo(28.7)
  })
})
