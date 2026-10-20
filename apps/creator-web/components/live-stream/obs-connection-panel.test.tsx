import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { beforeEach, describe, expect, it, vi } from "vitest"
import { ObsConnectionPanel } from "./obs-connection-panel"
import type { LiveStream } from "@/components/api-client"

const STREAM: LiveStream = {
  id: "ls_1",
  creatorId: "u_1",
  title: "Algebra II",
  description: "Quadratics",
  status: "waiting",
  statusLabel: "Waiting",
  streamKeyHint: "abcd1234",
  hasStreamKey: true,
  rtmpUrl: "rtmp://10.0.0.2:1935",
  playbackUrl: "http://10.0.0.2:8888/ls_1/index.m3u8",
  createdAt: "2026-10-20T10:00:00Z",
  updatedAt: "2026-10-20T10:00:00Z",
}

const RAW_KEY = "ls_1?token=0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

beforeEach(() => {
  vi.restoreAllMocks()
})

describe("ObsConnectionPanel", () => {
  it("masks the raw stream key by default and reveals it on toggle", async () => {
    const user = userEvent.setup()
    render(<ObsConnectionPanel stream={STREAM} rawStreamKey={RAW_KEY} />)

    const keyOutput = screen.getAllByText(/ls_1\?token/)[0]
    expect(keyOutput.textContent).not.toBe(RAW_KEY)
    expect(keyOutput.textContent).toContain("•")

    await user.click(screen.getByRole("button", { name: "Reveal stream key" }))
    expect(screen.getByText(RAW_KEY)).toBeInTheDocument()

    await user.click(screen.getByRole("button", { name: "Hide stream key" }))
    expect(screen.queryByText(RAW_KEY)).not.toBeInTheDocument()
  })

  it("copies the raw key even while it is masked", async () => {
    const user = userEvent.setup()
    const writeText = vi.spyOn(navigator.clipboard, "writeText")
    render(<ObsConnectionPanel stream={STREAM} rawStreamKey={RAW_KEY} />)

    const copyButtons = screen.getAllByRole("button", { name: /copy/i })
    await user.click(copyButtons[1])
    expect(writeText).toHaveBeenCalledWith(RAW_KEY)
  })

  it("shows only the hint and disables copy when the raw key is unavailable", () => {
    render(<ObsConnectionPanel stream={STREAM} rawStreamKey={null} />)

    expect(screen.getAllByText("abcd1234…").length).toBeGreaterThan(0)
    expect(screen.queryByRole("button", { name: "Reveal stream key" })).not.toBeInTheDocument()
    expect(screen.getAllByRole("button", { name: /copy/i })[1]).toBeDisabled()
  })

  it("renders the regenerate action when provided for an active stream", () => {
    const onRegenerate = vi.fn()
    render(
      <ObsConnectionPanel stream={STREAM} rawStreamKey={null} onRegenerate={onRegenerate} />,
    )
    expect(screen.getByRole("button", { name: "Regenerate stream key" })).toBeInTheDocument()
  })
})
