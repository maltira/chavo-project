import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render } from '@testing-library/react'
import type { ReactElement } from 'react'
import { MemoryRouter } from 'react-router'

// рендер с провайдерами приложения для тестов фич: запросы без повторов, роутер в памяти
export function renderWithProviders(ui: ReactElement, route = '/') {
    const client = new QueryClient({
        defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    })
    return render(
        <QueryClientProvider client={client}>
            <MemoryRouter initialEntries={[route]}>{ui}</MemoryRouter>
        </QueryClientProvider>,
    )
}

// набор текста в поле сообщения (contenteditable): браузер меняет содержимое и присылает input
export function typeMessage(field: HTMLElement, text: string) {
    field.textContent = text
    fireEvent.input(field)
}
