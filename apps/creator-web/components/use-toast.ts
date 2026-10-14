"use client"

import { toast } from "sonner"

export function useCreatorToast() {
  return {
    success: (message: string, description?: string) =>
      toast.success(message, { description }),
    error: (message: string, description?: string) =>
      toast.error(message, { description }),
    info: (message: string, description?: string) =>
      toast.info(message, { description }),
    warning: (message: string, description?: string) =>
      toast.warning(message, { description }),
  }
}
