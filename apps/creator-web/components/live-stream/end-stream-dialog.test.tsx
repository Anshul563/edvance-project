import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { describe, expect, it, vi } from "vitest"
import { EndStreamDialog } from "./end-stream-dialog"

describe("EndStreamDialog", () => {
  it("asks for confirmation and invokes onConfirm", async () => {
    const onConfirm = vi.fn()
    const user = userEvent.setup()
    render(
      <EndStreamDialog open onOpenChange={() => undefined} onConfirm={onConfirm} />,
    )

    expect(screen.getByText("End this live stream?")).toBeInTheDocument()
    await user.click(screen.getByRole("button", { name: "End Stream" }))
    expect(onConfirm).toHaveBeenCalledTimes(1)
  })

  it("disables both buttons while the end request is pending", async () => {
    const onConfirm = vi.fn()
    const user = userEvent.setup()
    render(
      <EndStreamDialog open onOpenChange={() => undefined} pending onConfirm={onConfirm} />,
    )

    expect(screen.getByRole("button", { name: "Ending…" })).toBeDisabled()
    expect(screen.getByRole("button", { name: "Close" })).toBeDisabled()
    await user.click(screen.getByRole("button", { name: "Ending…" }))
    expect(onConfirm).not.toHaveBeenCalled()
  })
})
