import fsd from '@feature-sliced/steiger-plugin'
import { defineConfig } from 'steiger'

export default defineConfig([
    ...fsd.configs.recommended,
    {
        files: ['./src/shared/**'],
        rules: { 'fsd/public-api': 'off' },
    },
    {
        // в app сегмент providers - обычное для FSD имя, правило про «назначение» его не принимает
        files: ['./src/app/**'],
        rules: { 'fsd/segments-by-purpose': 'off' },
    },
    {
        rules: { 'fsd/insignificant-slice': 'warn' },
    },
])
