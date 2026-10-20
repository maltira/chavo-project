import { useEffect, useRef } from 'react'

interface Pager {
    hasNextPage: boolean
    isFetchingNextPage: boolean
    isFetchNextPageError: boolean
    fetchNextPage: (options: { cancelRefetch: boolean }) => Promise<unknown>
}

// метка в конце списка: когда она подходит к видимой области, грузится следующая страница;
// после ошибки сам не повторяет - иначе запросы пошли бы по кругу
export function useLoadMore<T extends HTMLElement = HTMLDivElement>(pager: Pager, loaded: number) {
    const ref = useRef<T>(null)
    const { hasNextPage, isFetchingNextPage, isFetchNextPageError, fetchNextPage } = pager

    useEffect(() => {
        const node = ref.current
        if (
            !node ||
            !hasNextPage ||
            isFetchingNextPage ||
            isFetchNextPageError ||
            typeof IntersectionObserver === 'undefined'
        ) {
            return
        }
        const observer = new IntersectionObserver((entries) => {
            if (entries.some((entry) => entry.isIntersecting)) {
                void fetchNextPage({ cancelRefetch: false })
            }
        })
        observer.observe(node)
        return () => observer.disconnect()
        // loaded: после новой страницы метка могла остаться видимой - наблюдаем заново
    }, [hasNextPage, isFetchingNextPage, isFetchNextPageError, fetchNextPage, loaded])

    return ref
}
