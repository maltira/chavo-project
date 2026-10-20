import { useEffect, useState } from 'react'

interface Draft {
    blob: Blob
    url: string
}

// фото, выбранное до того, как его можно загрузить (профиль или группа ещё не созданы):
// хранит Blob и адрес для превью; адрес освобождается при замене фото и при размонтировании
export function usePhotoDraft() {
    const [draft, setDraft] = useState<Draft | null>(null)

    useEffect(
        () => () => {
            if (draft) {
                URL.revokeObjectURL(draft.url)
            }
        },
        [draft],
    )

    return {
        photo: draft?.blob ?? null,
        preview: draft?.url ?? null,
        set: (blob: Blob) => setDraft({ blob, url: URL.createObjectURL(blob) }),
        clear: () => setDraft(null),
    }
}
