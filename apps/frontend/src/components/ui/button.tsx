import * as React from 'react'
import { Slot } from '@radix-ui/react-slot'
import { cva, type VariantProps } from 'class-variance-authority'
import { cn } from '@/lib/utils'
const buttonVariants = cva('inline-flex items-center justify-center gap-2 whitespace-nowrap border-2 text-xs font-semibold transition-[transform,box-shadow,background-color] focus-visible:outline-2 focus-visible:outline-offset-4 focus-visible:outline-ring disabled:opacity-40 disabled:shadow-none [&_svg]:size-4', {
  variants: {
    variant: { default: 'border-primary bg-primary text-primary-foreground shadow-[3px_3px_0_#778540] enabled:hover:translate-x-[2px] enabled:hover:translate-y-[2px] enabled:hover:shadow-[1px_1px_0_#778540]', outline: 'border-input bg-background shadow-[3px_3px_0_#434936] enabled:hover:bg-accent enabled:hover:translate-x-[2px] enabled:hover:translate-y-[2px] enabled:hover:shadow-[1px_1px_0_#434936]', ghost: 'border-transparent enabled:hover:bg-accent enabled:hover:text-accent-foreground', destructive: 'border-destructive bg-destructive text-background enabled:hover:bg-destructive/90' },
    size: { default: 'h-10 px-4 py-2', sm: 'h-8 px-3 text-xs', icon: 'size-10' },
  }, defaultVariants: { variant: 'default', size: 'default' },
})
function Button({ className, variant, size, asChild = false, ...props }: React.ComponentProps<'button'> & VariantProps<typeof buttonVariants> & { asChild?: boolean }) {
  const Comp = asChild ? Slot : 'button'
  return <Comp data-slot="button" className={cn(buttonVariants({ variant, size, className }))} {...props} />
}
export { Button, buttonVariants }
