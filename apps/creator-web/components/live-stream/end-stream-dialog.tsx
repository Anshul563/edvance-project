"use client"

import { ConfirmationDialog } from "@/components/confirmation-dialog"

interface EndStreamDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  pending?: boolean
  onConfirm: () => void
}

export function EndStreamDialog({
  open,
  onOpenChange,
  pending = false,
  onConfirm,
}: EndStreamDialogProps) {
  return (
    <ConfirmationDialog
      open={open}
      onOpenChange={onOpenChange}
      title="End this live stream?"
      description="Students stop receiving the stream immediately and the OBS stream key stops working. This cannot be undone — create a new stream to broadcast again."
      confirmLabel={pending ? "Ending…" : "End Stream"}
      cancelLabel={pending ? "Close" : "Cancel"}
      confirmDisabled={pending}
      onConfirm={onConfirm}
    />
  )
}
