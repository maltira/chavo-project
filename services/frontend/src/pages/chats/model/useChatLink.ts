import { useQuery } from '@tanstack/react-query'
import { useEffect } from 'react'
import { useNavigate } from 'react-router'

import { getInviteInfo, previewFromInvite, useGroupCard } from '@/entities/conversation'
import { chatPath, ROUTES } from '@/shared/config'
import { notify } from '@/shared/lib'

// переход по ссылке на чат /chat/<код>: участник сразу попадает в чат, остальные - в окно группы
export function useChatLink(code: string | undefined) {
    const navigate = useNavigate()
    const openPreview = useGroupCard((state) => state.openPreview)
    const info = useQuery({
        queryKey: ['invite', code],
        queryFn: () => getInviteInfo(code!),
        enabled: !!code,
        retry: false,
        // ссылку могли сбросить, а заявку - рассмотреть: каждый переход - свежий ответ
        gcTime: 0,
    })

    useEffect(() => {
        if (!code) {
            return
        }
        if (info.data) {
            if (info.data.is_member) {
                navigate(chatPath(info.data.conversation_id), { replace: true })
                return
            }
            openPreview(previewFromInvite(info.data, code))
            navigate(ROUTES.home, { replace: true })
        } else if (info.isError) {
            notify({
                icon: 'warn',
                title: 'Ссылка недействительна',
                description: 'Возможно, её сбросили или группу удалили',
            })
            navigate(ROUTES.home, { replace: true })
        }
    }, [code, info.data, info.isError, navigate, openPreview])
}
