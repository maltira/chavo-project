import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'

import { App } from '@/app'
import { bootstrapSession } from '@/entities/session'
import { applyAccent, getInitialAccent } from '@/features/accent-color'
import { applyTheme, getInitialTheme } from '@/features/toggle-theme'

import '@/app/styles/index.scss'

// тема и акцентный цвет ставятся до первой отрисовки
applyTheme(getInitialTheme())
applyAccent(getInitialAccent())

// восстанавливаем сессию по refresh-cookie; пока идёт запрос, гарды показывают заставку
void bootstrapSession()

createRoot(document.getElementById('root')!).render(
    <StrictMode>
        <App />
    </StrictMode>,
)
