import { settleAll } from './settle'

describe('settleAll', () => {
    it('counts successes and failures and keeps the first error', async () => {
        const boom = new Error('404')
        const result = await settleAll([1, 2, 3], (n) =>
            n === 2 ? Promise.reject(boom) : Promise.resolve(),
        )
        expect(result).toEqual({ done: 2, failed: 1, firstError: boom })
    })

    it('reports nothing failed for an empty batch', async () => {
        expect(await settleAll([], () => Promise.resolve())).toEqual({
            done: 0,
            failed: 0,
            firstError: undefined,
        })
    })
})
