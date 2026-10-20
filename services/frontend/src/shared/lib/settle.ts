// итог пакетной операции (завершить сеансы, разблокировать нескольких): успехи и неудачи по отдельности
export interface SettleResult {
    done: number
    failed: number
    // причина первой неудачи - для текста уведомления
    firstError: unknown
}

// выполняет действие для каждого элемента и никогда не бросает: часть может удаться, часть нет
export async function settleAll<T>(
    items: T[],
    action: (item: T) => Promise<unknown>,
): Promise<SettleResult> {
    const results = await Promise.allSettled(items.map((item) => action(item)))
    const rejected = results.filter((r): r is PromiseRejectedResult => r.status === 'rejected')
    return {
        done: results.length - rejected.length,
        failed: rejected.length,
        firstError: rejected[0]?.reason,
    }
}
