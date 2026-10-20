// чей статус или фигура: мои или группы (меняет только её админ)
export type CustomTarget = { kind: 'me' } | { kind: 'group'; id: string }

export const ME: CustomTarget = { kind: 'me' }
