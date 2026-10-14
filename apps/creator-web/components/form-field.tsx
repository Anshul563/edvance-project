"use client"

import { cn } from "cn"
import { Label } from "@/components/ui/label"
import { Input } from "@/components/ui/input"
import { Textarea } from "@/components/ui/textarea"

interface FormFieldProps {
  id: string
  label: string
  type?: "text" | "email" | "password" | "number" | "textarea" | "select"
  placeholder?: string
  value?: string
  onChange?: (value: string) => void
  options?: { label: string; value: string }[]
  disabled?: boolean
  required?: boolean
  hint?: string
  error?: string
  className?: string
  rows?: number
}

export function FormField({
  id,
  label,
  type = "text",
  placeholder,
  value = "",
  onChange,
  options = [],
  disabled,
  required,
  hint,
  error,
  className,
  rows = 3,
}: FormFieldProps) {
  return (
    <div className={cn("flex flex-col gap-1", className)}>
      <Label htmlFor={id} className="text-xs font-medium">
        {label}
        {required && <span className="ml-0.5 text-destructive">*</span>}
      </Label>

      {type === "textarea" && (
        <Textarea
          id={id}
          placeholder={placeholder}
          value={value}
          onChange={(e) => onChange?.(e.target.value)}
          disabled={disabled}
          rows={rows}
          className={cn(error && "border-destructive")}
          aria-describedby={hint || error ? `${id}-hint` : undefined}
          aria-invalid={!!error}
        />
      )}

      {type === "select" && (
        <select
          id={id}
          value={value}
          onChange={(e) => onChange?.(e.target.value)}
          disabled={disabled}
          className={cn(
            "h-8 w-full rounded-none border border-border bg-background px-2 text-xs outline-none focus:border-ring focus:ring-1 focus:ring-ring/50 disabled:opacity-50",
            error && "border-destructive"
          )}
          aria-describedby={hint || error ? `${id}-hint` : undefined}
          aria-invalid={!!error}
        >
          {options.map((opt) => (
            <option key={opt.value} value={opt.value}>
              {opt.label}
            </option>
          ))}
        </select>
      )}

      {type !== "textarea" && type !== "select" && (
        <Input
          id={id}
          type={type}
          placeholder={placeholder}
          value={value}
          onChange={(e) => onChange?.(e.target.value)}
          disabled={disabled}
          className={cn(error && "border-destructive")}
          aria-describedby={hint || error ? `${id}-hint` : undefined}
          aria-invalid={!!error}
        />
      )}

      {(hint || error) && (
        <p
          id={`${id}-hint`}
          className={cn("text-xs", error ? "text-destructive" : "text-muted-foreground")}
        >
          {error ?? hint}
        </p>
      )}
    </div>
  )
}
