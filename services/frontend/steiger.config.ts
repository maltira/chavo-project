import fsd from '@feature-sliced/steiger-plugin'
import { defineConfig } from 'steiger'

export default defineConfig([
  ...fsd.configs.recommended,
  {
    // пока слои пустые - слайсов нет, часть правил не имеет смысла
    files: ['./src/shared/**'],
    rules: { 'fsd/public-api': 'off' },
  },
])
