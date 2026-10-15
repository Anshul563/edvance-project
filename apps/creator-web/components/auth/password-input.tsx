"use client"

import { useState } from "react"
import { EyeIcon, EyeOffIcon } from "@hugeicons/core-free-icons"
import { HugeiconsIcon } from "@hugeicons/react"
import { Input } from "@/components/ui/input"
import { cn } from "@/lib/utils"

type PasswordInputProps = Omit<React.ComponentProps<typeof Input>, "type">

export function PasswordInput({ className, ...props }: PasswordInputProps) {
  const [visible, setVisible] = useState(false)

  return (
    <div className="relative">
      <Input
        type={visible ? "text" : "password"}
        className={cn("pr-10", className)}
        {...props}
      />
      <button
        type="button"
        onClick={() => setVisible((current) => !current)}
        aria-label={visible ? "Hide password" : "Show password"}
        aria-pressed={visible}
        className="absolute inset-y-0 right-0 flex w-10 items-center justify-center text-muted-foreground transition-colors outline-none hover:text-foreground focus-visible:text-foreground"
      >
        <HugeiconsIcon
          icon={visible ? EyeOffIcon : EyeIcon}
          strokeWidth={1.75}
          className="size-4"
        />
      </button>
    </div>
  )
}
