import type { SVGProps } from 'react'

import LogotypeSvg from './Logotype.svg?react'

interface Props extends Omit<SVGProps<SVGSVGElement>, 'width' | 'height'> {
    width?: number | string
}

export function Logo({ width = 160, ...rest }: Props) {
    return <LogotypeSvg {...rest} width={width} height="auto" focusable="false" />
}
