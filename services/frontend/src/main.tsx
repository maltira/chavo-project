import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'

import { App } from '@/app'
import { bootstrapSession } from '@/entities/session'
import { applyTheme, getInitialTheme } from '@/features/toggle-theme'

import '@/app/styles/index.scss'

// тема ставится до первой отрисовки
applyTheme(getInitialTheme())

// восстанавливаем сессию по refresh-cookie; пока идёт запрос, гарды показывают заставку
void bootstrapSession()

createRoot(document.getElementById('root')!).render(
    <StrictMode>
        <App />
    </StrictMode>,
)
