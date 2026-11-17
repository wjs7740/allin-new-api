import type { PricingModel } from '../types'
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
import { evaluateBillingExpression } from './billing-expression/runtime'
import { withPluginPricing } from './plugin-pricing'
import { getTaskPricingDisplayTiers } from './task-matrix-display'

// Every displayed amount comes from the same expression the server executes.
export function mediaPriceRows(
  model: PricingModel
): { label: string; price: number; unit: string }[] {
  if (model.model_name.startsWith('gpt-image-')) {
    const specs = ['1K', '2K', '4K']
    if (model.model_name.startsWith('gpt-image-2.6')) specs.push('GIF')
    return specs.flatMap((spec) => {
      const result = evaluateBillingExpression(model.billing_expr || '', {
        imageCount: 1,
        request: {
          body: {
            resolution: spec.toLowerCase(),
            output_format: spec === 'GIF' ? 'gif' : 'png',
          },
        },
      })
      if (result.status !== 'success') return []
      // Include only branches that identify that specification. A flat price
      // continues through the existing ordinary-price presenter.
      if (result.matchedTier !== spec) return []
      return [{ label: spec, price: result.cost / 1000000, unit: 'image' }]
    })
  }
  const variant = model.billing_plugin_variants?.find(
    (item) => item.plugin_key === 'media' || item.plugin_key === 'zhiqi'
  )
  const selected = variant ? withPluginPricing(model, variant) : model
  if (
    !selected.billing_usage_schema ||
    !selected.supported_endpoint_types?.includes('openai-video')
  ) {
    return []
  }
  const tiers = getTaskPricingDisplayTiers(
    selected.billing_expr,
    selected.billing_usage_schema
  )
  return tiers
    .filter((tier) => tier.label !== 'default')
    .flatMap((tier) =>
      Object.entries(tier.unitPrices)
        .filter(([, value]) => value > 0)
        .map(([field, price]) => ({
          label: tier.label,
          // The shared task parser already returns token unit prices per 1M.
          price,
          unit:
            selected.billing_usage_schema?.[field]?.unit === 'token'
              ? '1M token'
              : 's',
        }))
    )
}
