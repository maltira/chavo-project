import { useInfiniteQuery } from '@tanstack/react-query'
import type { InfiniteData, QueryClient } from '@tanstack/react-query'

import type { ServerEvent } from '@/shared/api'

import { CONVERSATIONS_PAGE, getConversations } from '../api/conversations'
import { applyChatSettings, applyToConversations } from './events'
import type { ChatSettings, ConversationSummary } from './types'
import { conversationKey } from './useConversation'

// основной список; архив лежит под тем же префиксом, поэтому invalidate(conversationsKey) перечитывает оба
export const conversationsKey = ['conversations'] as const
export const archivedConversationsKey = ['conversations', 'archived'] as const

type Pages = InfiniteData<ConversationSummary[], number>

function useChatList(archived: boolean) {
    const query = useInfiniteQuery({
        queryKey: archived ? archivedConversationsKey : conversationsKey,
        queryFn: ({ pageParam }) => getConversations(pageParam, archived),
        initialPageParam: 0,
        // неполная страница - это конец списка; иначе следующая начинается после загруженных
        getNextPageParam: (last, all) =>
            last.length < CONVERSATIONS_PAGE ? undefined : all.flat().length,
    })

    return {
        ...query,
        // повтор возможен, когда событие подвинуло чат между страницами: оставляем первое вхождение
        data: query.data ? uniqueById(query.data.pages.flat()) : undefined,
    }
}

// список чатов без архивных: закреплённые сверху, дальше свежие; страницы подгружаются по мере прокрутки,
// между загрузками его обновляют события WebSocket
export function useConversations() {
    return useChatList(false)
}

// архив: чаты остаются в нём и после новых сообщений
export function useArchivedConversations() {
    return useChatList(true)
}

function uniqueById(list: ConversationSummary[]) {
    const seen = new Set<string>()
    return list.filter((c) => !seen.has(c.id) && seen.add(c.id))
}

function toPages(list: ConversationSummary[]): Pages {
    const pages: ConversationSummary[][] = []
    for (let i = 0; i < list.length; i += CONVERSATIONS_PAGE) {
        pages.push(list.slice(i, i + CONVERSATIONS_PAGE))
    }
    return { pages, pageParams: pages.map((_, i) => i * CONVERSATIONS_PAGE) }
}

function readList(queryClient: QueryClient, key: readonly string[]) {
    const cached = queryClient.getQueryData<Pages>(key)
    return cached ? uniqueById(cached.pages.flat()) : undefined
}

// чат из загруженных списков (основного или архива)
export function findInLists(queryClient: QueryClient, id: string) {
    return (
        readList(queryClient, conversationsKey)?.find((c) => c.id === id) ??
        readList(queryClient, archivedConversationsKey)?.find((c) => c.id === id)
    )
}

// событие меняет списки в кеше: новое сообщение поднимает чат, прочтение ставит галочки,
// новые личные настройки переставляют чат или переносят его в архив и обратно
export function applyConversationEvent(
    queryClient: QueryClient,
    event: ServerEvent,
    myId: string | undefined,
) {
    const main = readList(queryClient, conversationsKey)
    const archived = readList(queryClient, archivedConversationsKey)

    if (event.type === 'conversation.settings.updated') {
        const data = (event.data ?? {}) as Partial<ChatSettings> & { conversation_id?: string }
        if (data.conversation_id) {
            // карточка открытого чата (меню в шапке) - тоже с новыми настройками
            queryClient.setQueryData<ConversationSummary>(
                conversationKey(data.conversation_id),
                (chat) =>
                    chat && {
                        ...chat,
                        muted_until: data.muted_until ?? null,
                        archived_at: data.archived_at ?? null,
                        pinned_at: data.pinned_at ?? null,
                    },
            )
        }
        const update = applyChatSettings(main, archived, event)
        if (update.reload) {
            void queryClient.invalidateQueries({ queryKey: conversationsKey })
            return
        }
        if (update.main) {
            queryClient.setQueryData<Pages>(conversationsKey, toPages(update.main))
        }
        if (update.archived) {
            queryClient.setQueryData<Pages>(archivedConversationsKey, toPages(update.archived))
        } else {
            // архив ещё не загружен: о новом архивном чате он узнает при загрузке
            void queryClient.invalidateQueries({ queryKey: archivedConversationsKey })
        }
        return
    }

    // событие архивного чата меняет архив, остальные - основной список
    const chatId = (event.data as { conversation_id?: string } | undefined)?.conversation_id
    const inArchive = !!archived?.some((c) => c.id === chatId)
    const key = inArchive ? archivedConversationsKey : conversationsKey
    const update = applyToConversations(inArchive ? archived : main, event, myId)

    if (update.kind === 'list') {
        queryClient.setQueryData<Pages>(key, toPages(update.list))
    } else if (update.kind === 'invalidate') {
        void queryClient.invalidateQueries({ queryKey: conversationsKey })
    }
}

// личные настройки в кеше сразу после ответа сервера; событие для остальных вкладок придёт следом
export function setChatSettings(queryClient: QueryClient, id: string, settings: ChatSettings) {
    applyConversationEvent(
        queryClient,
        { type: 'conversation.settings.updated', data: { conversation_id: id, ...settings } },
        undefined,
    )
}

// личный чат с человеком среди загруженных; если его нет, чат создаст первое сообщение
export function findDirectWith(queryClient: QueryClient, userId: string) {
    return [
        ...(readList(queryClient, conversationsKey) ?? []),
        ...(readList(queryClient, archivedConversationsKey) ?? []),
    ].find((c) => c.conversation_type === 'direct' && c.peer_id === userId)
}

// «Избранное» среди загруженных; если его нет, его создаст первое сообщение себе
export function findSaved(queryClient: QueryClient) {
    return [
        ...(readList(queryClient, conversationsKey) ?? []),
        ...(readList(queryClient, archivedConversationsKey) ?? []),
    ].find((c) => c.is_saved)
}

// собеседники из моих личных чатов (загруженных): их можно позвать в группу
export function useDirectPeers() {
    const chats = useConversations()
    return (chats.data ?? [])
        .filter((chat) => chat.conversation_type === 'direct' && chat.peer_id)
        .map((chat) => chat.peer_id!)
}
