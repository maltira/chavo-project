import { act, render, screen } from '@testing-library/react'

import { Splash } from './Splash'

describe('Splash', () => {
    beforeEach(() => {
        vi.useFakeTimers()
    })

    afterEach(() => {
        vi.useRealTimers()
    })

    it('starts with the friendly text under the loader', () => {
        render(<Splash since={Date.now()} />)
        expect(screen.getByTestId('splash-text')).toHaveTextContent('Подготавливаем ваши чаты')
    })

    it('changes the text as the wait gets longer', () => {
        render(<Splash since={Date.now()} />)

        act(() => vi.advanceTimersByTime(6_000))
        expect(screen.getByTestId('splash-text')).toHaveTextContent(
            'Сегодня чуть медленнее, чем обычно',
        )

        act(() => vi.advanceTimersByTime(9_000))
        expect(screen.getByTestId('splash-text')).toHaveTextContent('Всё ещё загружаем')

        act(() => vi.advanceTimersByTime(15_000))
        expect(screen.getByTestId('splash-text')).toHaveTextContent('попробуйте обновить страницу')
    })

    it('counts from the app start, not from the moment of mounting', () => {
        render(<Splash since={Date.now() - 20_000} />)
        expect(screen.getByTestId('splash-text')).toHaveTextContent('Всё ещё загружаем')
    })

    it('jumps to the slow text when the request is being retried', () => {
        render(<Splash since={Date.now()} retrying />)
        expect(screen.getByTestId('splash-text')).toHaveTextContent('Сегодня чуть медленнее')
    })

    it('shows the last text at once when loading failed', () => {
        render(<Splash since={Date.now()} failed />)
        expect(screen.getByTestId('splash-text')).toHaveTextContent('попробуйте обновить страницу')
    })
})
