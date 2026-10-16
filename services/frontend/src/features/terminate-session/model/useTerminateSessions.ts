import { useMutation, useQueryClient } from '@tanstack/react-query'

import { sessionsKey, terminateSession } from '@/entities/session'
import { errorMessage } from '@/shared/api'
import { notify, settleAll } from '@/shared/lib'

// завершение чужих сеансов: одного или всех разом (отдельного запроса «все, кроме текущего» нет);
// каждый по своему запросу, поэтому часть может не завершиться (например, уже истекла)
export function useTerminateSessions() {
    const queryClient = useQueryClient()

    return useMutation({
        mutationFn: (ids: string[]) => settleAll(ids, terminateSession),
        onSettled: () => queryClient.invalidateQueries({ queryKey: sessionsKey }),
        onSuccess: ({ done, failed, firstError }, ids) => {
            if (failed === 0) {
                notify({
                    icon: 'check',
                    title: ids.length > 1 ? 'Сеансы завершены' : 'Сеанс завершён',
                })
            } else if (done > 0) {
                notify({
                    icon: 'warn',
                    title: `Завершено ${done} из ${ids.length}`,
                    description: errorMessage(firstError),
                })
            } else {
                notify({
                    icon: 'warn',
                    title: 'Не получилось',
                    description: errorMessage(firstError),
                })
            }
        },
    })
}
