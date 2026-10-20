import { useEffect, useState } from 'react'

// значение, которое догоняет исходное после паузы: запрос уходит, когда перестали печатать
export function useDebouncedValue<T>(value: T, delay = 300): T {
    const [debounced, setDebounced] = useState(value)
    useEffect(() => {
        const timer = setTimeout(() => setDebounced(value), delay)
        return () => clearTimeout(timer)
    }, [value, delay])
    return debounced
}
