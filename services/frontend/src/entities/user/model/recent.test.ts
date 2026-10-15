import { RECENT_LIMIT, useRecentUsers } from './recent'

describe('recent users', () => {
    beforeEach(() => {
        localStorage.clear()
        useRecentUsers.setState({ ids: [] })
    })

    it('keeps the freshest first without duplicates and saves to localStorage', () => {
        const { add } = useRecentUsers.getState()
        add('a')
        add('b')
        add('a')
        expect(useRecentUsers.getState().ids).toEqual(['a', 'b'])
        expect(JSON.parse(localStorage.getItem('chavo:recent-users')!)).toEqual(['a', 'b'])
    })

    it('remembers at most the limit and clears', () => {
        const { add, clear } = useRecentUsers.getState()
        for (let i = 0; i < RECENT_LIMIT + 3; i++) {
            add(`u${i}`)
        }
        expect(useRecentUsers.getState().ids).toHaveLength(RECENT_LIMIT)
        clear()
        expect(useRecentUsers.getState().ids).toEqual([])
    })

    it('works when localStorage throws', () => {
        vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
            throw new Error('denied')
        })
        useRecentUsers.getState().add('x')
        expect(useRecentUsers.getState().ids).toEqual(['x'])
        vi.restoreAllMocks()
    })
})
