import * as React from 'react'
import { cn } from '@/lib/utils'
export function Card({className,...props}:React.ComponentProps<'section'>){return <section data-slot="card" className={cn('rounded-xl border bg-card text-card-foreground shadow-sm',className)} {...props}/>}
export function CardHeader({className,...props}:React.ComponentProps<'div'>){return <div data-slot="card-header" className={cn('p-6 pb-4',className)} {...props}/>}
export function CardTitle({className,...props}:React.ComponentProps<'h2'>){return <h2 data-slot="card-title" className={cn('font-semibold tracking-tight',className)} {...props}/>}
export function CardContent({className,...props}:React.ComponentProps<'div'>){return <div data-slot="card-content" className={cn('px-6 pb-6',className)} {...props}/>}
