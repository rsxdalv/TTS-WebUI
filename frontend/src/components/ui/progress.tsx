"use client"

import * as React from "react"
import * as ProgressPrimitive from "@radix-ui/react-progress"

import { cn } from "@/lib/utils"

const Progress = React.forwardRef<
  React.ElementRef<typeof ProgressPrimitive.Root>,
  React.ComponentPropsWithoutRef<typeof ProgressPrimitive.Root>
>(({ className, value, max, ...props }, ref) => (
  <ProgressPrimitive.Root
    ref={ref}
    className={cn(
      "relative h-4 w-full overflow-hidden rounded-full bg-secondary",
      // "relative h-4 w-full overflow-hidden rounded-full bg-slate-300",
      className
    )}
    value={value}
    max={max}
    {...props}
  >
    <ProgressPrimitive.Indicator
      // className="h-full w-full flex-1 bg-primary transition-all"
      className="h-full w-full flex-1 bg-orange-400 transition-all"
      style={{ transform: `translateX(-${100 - ((value || 0) / (max || 100)) * 100}%)` }}
    />
  </ProgressPrimitive.Root>
))
Progress.displayName = ProgressPrimitive.Root.displayName

export { Progress }
