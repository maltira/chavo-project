import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router'

import { FaqPage, PrivacyPage, TermsPage } from './InfoPage'

describe('info pages', () => {
    it.each([
        [FaqPage, 'FAQ и поддержка'],
        [TermsPage, 'Условия использования'],
        [PrivacyPage, 'Политика конфиденциальности'],
    ])('shows the title and a way back to the app', (Page, title) => {
        render(
            <MemoryRouter>
                <Page />
            </MemoryRouter>,
        )
        expect(screen.getByRole('heading', { name: title })).toBeInTheDocument()
        expect(screen.getByRole('button', { name: 'В приложение' })).toBeInTheDocument()
    })
})
