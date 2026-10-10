import * as React from 'react'
import { cn } from '@/lib/utils'
export function Input({ className, type, ...props }: React.ComponentProps<'input'>) {
  return <input type={type} data-slot="input" className={cn('flex h-11 w-full min-w-0 border border-input bg-background px-3 py-2 text-xs placeholder:text-muted-foreground focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring disabled:opacity-50', className)} {...props} />
}
