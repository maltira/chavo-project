import { fireEvent, render, screen } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router'

import { FaqPage, PrivacyPage, TermsPage } from './InfoPage'

function renderAt(path: string) {
    render(
        <MemoryRouter initialEntries={[path]}>
            <Routes>
                <Route path="/faq" element={<FaqPage />} />
                <Route path="/terms" element={<TermsPage />} />
                <Route path="/privacy" element={<PrivacyPage />} />
                <Route path="/" element={<p>приложение</p>} />
            </Routes>
        </MemoryRouter>,
    )
}

describe('info pages', () => {
    it('shows FAQ topics with questions that open on click', () => {
        renderAt('/faq')
        expect(
            screen.getByRole('heading', { level: 1, name: 'FAQ и поддержка' }),
        ).toBeInTheDocument()
        expect(screen.getByRole('heading', { name: 'Группы' })).toBeInTheDocument()

        const question = screen.getByText('Что значат галочки?').closest('details')!
        expect(question).not.toHaveAttribute('open')
        fireEvent.click(screen.getByText('Что значат галочки?'))
        expect(question).toHaveAttribute('open')
    })

    it('switches between the sections with the tabs', () => {
        renderAt('/faq')
        fireEvent.click(screen.getByRole('radio', { name: 'Условия' }))
        expect(
            screen.getByRole('heading', { level: 1, name: 'Условия использования' }),
        ).toBeInTheDocument()
        expect(screen.getByText(/Редакция от/)).toBeInTheDocument()
        expect(screen.getByRole('heading', { name: '1. Общие положения' })).toBeInTheDocument()

        fireEvent.click(screen.getByRole('radio', { name: 'Конфиденциальность' }))
        expect(
            screen.getByRole('heading', { level: 1, name: 'Политика конфиденциальности' }),
        ).toBeInTheDocument()
    })
})
