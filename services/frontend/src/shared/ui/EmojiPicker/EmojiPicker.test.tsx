import { fireEvent, render, screen, within } from '@testing-library/react'

import { EmojiPicker } from './EmojiPicker'

function setup(favorites: string[] = []) {
    const onPick = vi.fn()
    const onToggleFavorite = vi.fn()
    render(
        <EmojiPicker onPick={onPick} favorites={favorites} onToggleFavorite={onToggleFavorite} />,
    )
    return { onPick, onToggleFavorite }
}

// каталог грузится лениво: ждём первую секцию
const ready = () => screen.findByRole('region', { name: 'Смайлы и люди' })

describe('EmojiPicker', () => {
    beforeEach(() => localStorage.clear())

    it('loads the catalog and shows the categories with their tabs', async () => {
        setup()
        await ready()
        for (const name of ['Избранные', 'Животные и природа', 'Еда и напитки', 'Флаги']) {
            expect(screen.getByRole('button', { name })).toBeInTheDocument()
            expect(screen.getByRole('region', { name })).toBeInTheDocument()
        }
        // «Часто используемых» ещё нет - ни секции, ни вкладки
        expect(screen.queryByRole('region', { name: 'Часто используемые' })).toBeNull()
        // у пустого избранного - подсказка, как добавить
        expect(screen.getByText(/Правая кнопка мыши по эмодзи/)).toBeInTheDocument()
        expect(screen.getByLabelText('Поиск эмодзи')).toHaveFocus()
    })

    it('picks an emoji, keeps the panel open and remembers it as frequent', async () => {
        const { onPick } = setup()
        await ready()
        const people = screen.getByRole('region', { name: 'Смайлы и люди' })
        fireEvent.click(within(people).getAllByRole('button')[0]!)
        expect(onPick).toHaveBeenCalledTimes(1)
        const emoji = onPick.mock.calls[0]![0] as string
        expect(JSON.parse(localStorage.getItem('chavo:emoji-frequent')!)).toEqual({ [emoji]: 1 })
        expect(screen.getByRole('dialog', { name: 'Эмодзи' })).toBeInTheDocument()
    })

    it('shows frequent emojis first, most used on top', async () => {
        localStorage.setItem('chavo:emoji-frequent', JSON.stringify({ '🔥': 2, '😂': 9 }))
        setup()
        const frequent = await screen.findByRole('region', { name: 'Часто используемые' })
        expect(
            within(frequent)
                .getAllByRole('button')
                .map((b) => b.textContent),
        ).toEqual(['😂', '🔥'])
        expect(screen.getByRole('button', { name: 'Часто используемые' })).toBeInTheDocument()
    })

    it('searches by the Russian name and keywords, all words must match', async () => {
        const { onPick } = setup()
        await ready()
        const search = screen.getByLabelText('Поиск эмодзи')
        fireEvent.change(search, { target: { value: 'кошка' } })
        const results = screen.getByRole('region', { name: 'Результаты поиска' })
        expect(within(results).getAllByRole('button').length).toBeGreaterThan(0)
        // во время поиска категорий и вкладок нет
        expect(screen.queryByRole('region', { name: 'Смайлы и люди' })).toBeNull()
        expect(screen.queryByRole('toolbar', { name: 'Категории' })).toBeNull()

        fireEvent.click(within(results).getAllByRole('button')[0]!)
        expect(onPick).toHaveBeenCalled()

        fireEvent.change(search, { target: { value: 'ъъъъ нет такого' } })
        expect(screen.getByText('Ничего не найдено')).toBeInTheDocument()
        // очистка возвращает каталог
        fireEvent.change(search, { target: { value: '' } })
        expect(await ready()).toBeInTheDocument()
    })

    it('shows favorites and toggles one with the right mouse button', async () => {
        const { onToggleFavorite, onPick } = setup(['🔥', '👍'])
        const favorites = await screen.findByRole('region', { name: 'Избранные' })
        expect(
            within(favorites)
                .getAllByRole('button')
                .map((b) => b.textContent),
        ).toEqual(['🔥', '👍'])

        fireEvent.contextMenu(within(favorites).getAllByRole('button')[0]!)
        expect(onToggleFavorite).toHaveBeenCalledWith('🔥')
        // выбор эмодзи правой кнопкой не вставляет его
        expect(onPick).not.toHaveBeenCalled()
        // и из обычной секции можно добавить в избранное
        const people = screen.getByRole('region', { name: 'Смайлы и люди' })
        fireEvent.contextMenu(within(people).getAllByRole('button')[1]!)
        expect(onToggleFavorite).toHaveBeenCalledTimes(2)
    })

    it('moves the focus over the grid with the arrow keys', async () => {
        setup()
        await ready()
        const people = screen.getByRole('region', { name: 'Смайлы и люди' })
        const [first, second] = within(people).getAllByRole('button') as HTMLElement[]
        first!.focus()
        fireEvent.keyDown(first!, { key: 'ArrowRight' })
        expect(second).toHaveFocus()
        fireEvent.keyDown(second!, { key: 'ArrowLeft' })
        expect(first).toHaveFocus()
        // вниз из поля поиска - к первому эмодзи
        fireEvent.keyDown(screen.getByLabelText('Поиск эмодзи'), { key: 'ArrowDown' })
        expect(document.activeElement?.getAttribute('data-emoji')).toBeTruthy()
    })

    it('works without the browser storage', async () => {
        const getItem = vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => {
            throw new Error('blocked')
        })
        const setItem = vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
            throw new Error('blocked')
        })
        const { onPick } = setup()
        const people = await ready()
        fireEvent.click(within(people).getAllByRole('button')[0]!)
        expect(onPick).toHaveBeenCalled()
        getItem.mockRestore()
        setItem.mockRestore()
    })
})
