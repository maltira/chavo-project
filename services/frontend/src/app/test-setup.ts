import '@testing-library/jest-dom/vitest'
import { configure } from '@testing-library/react'

// при полном прогоне воркеры делят процессор, и первая загрузка каталога эмодзи (большой JSON) идёт дольше
// секунды: ожидания findBy/waitFor по умолчанию слишком короткие
configure({ asyncUtilTimeout: 5000 })
