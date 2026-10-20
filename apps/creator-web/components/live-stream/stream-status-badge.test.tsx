import { render, screen } from "@testing-library/react"
import { describe, expect, it } from "vitest"
import { StreamStatusBadge } from "./stream-status-badge"

describe("StreamStatusBadge", () => {
  it("prefers the server-provided status label", () => {
    render(<StreamStatusBadge status="live" statusLabel="On Air" />)
    expect(screen.getByText("On Air")).toBeInTheDocument()
  })

  it("falls back to a local label when none is provided", () => {
    const { rerender } = render(<StreamStatusBadge status="waiting" />)
    expect(screen.getByText("Waiting")).toBeInTheDocument()
    rerender(<StreamStatusBadge status="ended" />)
    expect(screen.getByText("Ended")).toBeInTheDocument()
  })
})
