import { fireEvent, render, screen } from '@testing-library/react'

import { RichText } from './RichText'

describe('RichText', () => {
    it('renders plain text as is', () => {
        const { container } = render(<RichText text="просто текст" />)
        expect(container.innerHTML).toBe('просто текст')
        expect(render(<RichText text="и так" entities={[]} />).container.innerHTML).toBe('и так')
    })

    it('wraps formatted parts and nests the formats that overlap', () => {
        const { container } = render(
            <RichText
                text="привет мир"
                entities={[
                    { type: 'bold', offset: 0, length: 6 },
                    { type: 'italic', offset: 3, length: 7 },
                    { type: 'underline', offset: 7, length: 3 },
                ]}
            />,
        )
        expect(container.textContent).toBe('привет мир')
        expect(Array.from(container.querySelectorAll('strong'), (n) => n.textContent)).toEqual([
            'при',
            'вет',
        ])
        expect(Array.from(container.querySelectorAll('em'), (n) => n.textContent)).toEqual([
            'вет',
            ' ',
            'мир',
        ])
        // жирный и курсив вместе - вложенные теги
        expect(container.querySelector('strong em')?.textContent).toBe('вет')
        expect(container.querySelector('em u')?.textContent).toBe('мир')
    })

    it('renders strikethrough, code, spoiler, links and mentions interactively', () => {
        const onMention = vi.fn()
        const { container } = render(
            <RichText
                text="strike code secret ссылка @bob https://example.com"
                entities={[
                    { type: 'strikethrough', offset: 0, length: 6 },
                    { type: 'code', offset: 7, length: 4 },
                    { type: 'spoiler', offset: 12, length: 6 },
                    {
                        type: 'text_link',
                        offset: 19,
                        length: 6,
                        url: 'https://chavo.example',
                    },
                    { type: 'mention', offset: 26, length: 4, user_id: 'u-bob' },
                ]}
                onMentionClick={onMention}
            />,
        )
        expect(container.querySelector('s')?.textContent).toBe('strike')
        expect(container.querySelector('code')?.textContent).toBe('code')

        // спойлер покрыт мозаикой и открывается по клику
        const spoiler = screen.getByRole('button', { name: 'Показать скрытый текст' })
        expect(spoiler.querySelectorAll('[aria-hidden="true"] span').length).toBeGreaterThanOrEqual(
            8,
        )
        fireEvent.click(spoiler)
        expect(screen.queryByRole('button', { name: 'Показать скрытый текст' })).toBeNull()

        // явная text_link и автолинкованная ссылка
        const links = container.querySelectorAll('a')
        expect(links).toHaveLength(2)
        expect(links[0]).toHaveAttribute('href', 'https://chavo.example')
        expect(links[0]).toHaveAttribute('rel', 'noopener noreferrer')
        expect(links[1]).toHaveAttribute('href', 'https://example.com')

        // клик по упоминанию вызывает обработчик профиля
        fireEvent.click(screen.getByRole('button', { name: '@bob' }))
        expect(onMention).toHaveBeenCalledWith('u-bob')
    })

    it('renders code blocks with language label and copy button', () => {
        const writeText = vi.fn().mockResolvedValue(undefined)
        Object.assign(navigator, { clipboard: { writeText } })

        render(
            <RichText
                text="fmt.Println(1)"
                entities={[{ type: 'pre', offset: 0, length: 14, language: 'go' }]}
            />,
        )
        expect(screen.getByText('Go')).toBeInTheDocument()
        fireEvent.click(screen.getByRole('button', { name: 'Копировать код' }))
        expect(writeText).toHaveBeenCalledWith('fmt.Println(1)')
        expect(screen.getByText('Скопировано')).toBeInTheDocument()
    })

    it('keeps a code block and a spoiler as single blocks when nested formatting splits segments', () => {
        const writeText = vi.fn().mockResolvedValue(undefined)
        Object.assign(navigator, { clipboard: { writeText } })

        const code = 'const a = 1\nconst b = 2'
        const { container } = render(
            <RichText
                text={code}
                entities={[
                    { type: 'pre', offset: 0, length: code.length, language: 'typescript' },
                    { type: 'bold', offset: 6, length: 5 },
                ]}
            />,
        )
        // один блок кода с одной кнопкой копирования, которая копирует весь блок целиком
        const copyButtons = screen.getAllByRole('button', { name: 'Копировать код' })
        expect(copyButtons).toHaveLength(1)
        expect(container.querySelector('strong')?.textContent).toBe('a = 1')
        fireEvent.click(copyButtons[0]!)
        expect(writeText).toHaveBeenCalledWith(code)

        // вложенный bold внутри спойлера тоже остаётся одной кнопкой спойлера
        const spoilerView = render(
            <RichText
                text="тайный текст"
                entities={[
                    { type: 'spoiler', offset: 0, length: 12 },
                    { type: 'bold', offset: 0, length: 6 },
                ]}
            />,
        )
        const spoilerBtns = spoilerView.getAllByRole('button', { name: 'Показать скрытый текст' })
        expect(spoilerBtns).toHaveLength(1)
        fireEvent.click(spoilerBtns[0]!)
        expect(spoilerView.queryByRole('button', { name: 'Показать скрытый текст' })).toBeNull()
    })

    it('renders non-interactive spans when interactive is false', () => {
        const { container } = render(
            <RichText
                text="ссылка @bob"
                entities={[
                    { type: 'text_link', offset: 0, length: 6, url: 'https://example.com' },
                    { type: 'mention', offset: 7, length: 4, user_id: 'u-bob' },
                ]}
                interactive={false}
                onMentionClick={vi.fn()}
            />,
        )
        expect(container.querySelector('a')).toBeNull()
        expect(container.querySelector('button')).toBeNull()
    })
})
