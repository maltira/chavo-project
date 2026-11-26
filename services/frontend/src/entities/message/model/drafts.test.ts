import { getDraft, useDrafts } from './drafts'

describe('useDrafts', () => {
    beforeEach(() => {
        localStorage.clear()
        useDrafts.getState().clearAll()
    })

    it('saves and persists draft text, entities, and reply per chat', () => {
        useDrafts.getState().setDraft('c1', {
            text: 'привет',
            entities: [{ type: 'bold', offset: 0, length: 6 }],
            replyTo: { id: 'm1', author: 'Дорофей', text: 'вопрос' },
        })
        expect(getDraft('c1')).toEqual({
            text: 'привет',
            entities: [{ type: 'bold', offset: 0, length: 6 }],
            replyTo: { id: 'm1', author: 'Дорофей', text: 'вопрос' },
        })
        expect(JSON.parse(localStorage.getItem('chavo:drafts')!)).toHaveProperty('c1.text', 'привет')
    })

    it('drops draft when text is blank and there is no reply', () => {
        useDrafts.getState().setDraft('c1', { text: 'черновик', entities: [] })
        expect(getDraft('c1')).toBeDefined()

        useDrafts.getState().setDraft('c1', { text: '   ', entities: [], replyTo: null })
        expect(getDraft('c1')).toBeUndefined()
        expect(localStorage.getItem('chavo:drafts')).toBeNull()
    })
})
