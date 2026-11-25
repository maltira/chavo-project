import { render } from '@testing-library/react'

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
})
