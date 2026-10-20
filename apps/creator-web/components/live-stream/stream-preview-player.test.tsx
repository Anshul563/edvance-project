import { render, screen, act, fireEvent } from "@testing-library/react"
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"
import Hls from "hls.js"
import { StreamPreviewPlayer } from "./stream-preview-player"

const { FakeHls } = vi.hoisted(() => {
  type Listener = (event: unknown, data?: unknown) => void

  class FakeHls {
    static supported = true
    static instances: FakeHls[] = []
    static Events = {
      MANIFEST_PARSED: "hlsManifestParsed",
      ERROR: "hlsError",
      FRAG_BUFFERED: "hlsFragBuffered",
    }
    static ErrorTypes = {
      NETWORK_ERROR: "networkError",
      MEDIA_ERROR: "mediaError",
    }
    static isSupported() {
      return FakeHls.supported
    }

    listeners = new Map<string, Set<Listener>>()
    loadSource = vi.fn()
    attachMedia = vi.fn()
    recoverMediaError = vi.fn()
    destroy = vi.fn()

    constructor(public config: Record<string, unknown>) {
      FakeHls.instances.push(this)
    }

    on(event: string, listener: Listener) {
      let set = this.listeners.get(event)
      if (!set) {
        set = new Set()
        this.listeners.set(event, set)
      }
      set.add(listener)
    }

    emit(event: string, data?: unknown) {
      this.listeners.get(event)?.forEach((listener) => listener({ type: event }, data))
    }
  }

  return { FakeHls }
})

vi.mock("hls.js", () => ({
  default: FakeHls,
}))

const PLAYBACK_URL = "http://media.test/ls_abc/index.m3u8"

type FakeHlsInstance = InstanceType<typeof FakeHls>

function lastInstance(): FakeHlsInstance {
  const instance = FakeHls.instances.at(-1)
  if (!instance) throw new Error("no Hls instance created")
  return instance
}

function emitFatalNetwork(hls: FakeHlsInstance) {
  act(() => {
    hls.emit(Hls.Events.ERROR, { fatal: true, type: Hls.ErrorTypes.NETWORK_ERROR })
  })
}

beforeEach(() => {
  FakeHls.supported = true
  FakeHls.instances = []
})

afterEach(() => {
  vi.useRealTimers()
})

describe("StreamPreviewPlayer", () => {
  it("shows waiting-for-OBS state without initializing HLS", () => {
    render(
      <StreamPreviewPlayer playbackUrl={PLAYBACK_URL} status="waiting" statusLabel="Waiting" />,
    )
    expect(screen.getByText("Waiting for OBS")).toBeInTheDocument()
    expect(FakeHls.instances).toHaveLength(0)
  })

  it("shows the ended state without initializing HLS", () => {
    render(<StreamPreviewPlayer playbackUrl={PLAYBACK_URL} status="ended" statusLabel="Ended" />)
    expect(screen.getByText("Stream ended")).toBeInTheDocument()
    expect(FakeHls.instances).toHaveLength(0)
  })

  it("initializes hls.js with the playback URL when live", () => {
    render(<StreamPreviewPlayer playbackUrl={PLAYBACK_URL} status="live" statusLabel="Live" />)
    const hls = lastInstance()
    expect(hls.loadSource).toHaveBeenCalledWith(PLAYBACK_URL)
    expect(hls.attachMedia).toHaveBeenCalledTimes(1)
  })

  it("cleans up the HLS instance on unmount", () => {
    const { unmount } = render(
      <StreamPreviewPlayer playbackUrl={PLAYBACK_URL} status="live" statusLabel="Live" />,
    )
    const hls = lastInstance()
    unmount()
    expect(hls.destroy).toHaveBeenCalled()
  })

  it("auto-retries fatal network errors, surfaces interrupted, and recovers via Retry", () => {
    vi.useFakeTimers()
    render(<StreamPreviewPlayer playbackUrl={PLAYBACK_URL} status="live" statusLabel="Live" />)
    expect(FakeHls.instances).toHaveLength(1)

    // Each fatal network error shows "waiting for media", then a new
    // engine instance is created after the retry delay.
    for (let attempt = 0; attempt < 8; attempt += 1) {
      emitFatalNetwork(lastInstance())
      act(() => {
        vi.advanceTimersByTime(3000)
      })
    }
    expect(FakeHls.instances).toHaveLength(9)

    // The 9th fatal error exhausts the budget.
    emitFatalNetwork(lastInstance())
    expect(screen.getByText("Stream interrupted")).toBeInTheDocument()

    const beforeRetry = FakeHls.instances.length
    fireEvent.click(screen.getByRole("button", { name: "Retry" }))
    expect(FakeHls.instances).toHaveLength(beforeRetry + 1)
    expect(lastInstance().loadSource).toHaveBeenCalledWith(PLAYBACK_URL)
  })

  it("auto-retries once for a recoverable media error, then fails", () => {
    render(<StreamPreviewPlayer playbackUrl={PLAYBACK_URL} status="live" statusLabel="Live" />)
    const hls = lastInstance()

    act(() => {
      hls.emit(Hls.Events.ERROR, { fatal: true, type: Hls.ErrorTypes.MEDIA_ERROR })
    })
    expect(hls.recoverMediaError).toHaveBeenCalledTimes(1)
    expect(screen.queryByText("Playback error")).not.toBeInTheDocument()

    act(() => {
      hls.emit(Hls.Events.ERROR, { fatal: true, type: Hls.ErrorTypes.MEDIA_ERROR })
    })
    expect(screen.getByText("Playback error")).toBeInTheDocument()
    expect(hls.destroy).toHaveBeenCalled()
  })

  it("uses native HLS when hls.js is unsupported", () => {
    FakeHls.supported = false
    const canPlay = vi
      .spyOn(HTMLMediaElement.prototype, "canPlayType")
      .mockReturnValue("maybe" as CanPlayTypeResult)
    try {
      const { container } = render(
        <StreamPreviewPlayer playbackUrl={PLAYBACK_URL} status="live" statusLabel="Live" />,
      )
      const video = container.querySelector("video")
      expect(video).toHaveAttribute("src", PLAYBACK_URL)
      expect(FakeHls.instances).toHaveLength(0)
      expect(screen.getByText(/waiting for the first video segments/i)).toBeInTheDocument()
      fireEvent(video as HTMLVideoElement, new Event("loadedmetadata"))
      expect(screen.getByText("Click to play the live preview")).toBeInTheDocument()
    } finally {
      canPlay.mockRestore()
    }
  })

  it("switches to the paused overlay when the video pauses while playing", () => {
    const { container } = render(
      <StreamPreviewPlayer playbackUrl={PLAYBACK_URL} status="live" statusLabel="Live" />,
    )
    const video = container.querySelector("video") as HTMLVideoElement

    act(() => {
      lastInstance().emit(Hls.Events.MANIFEST_PARSED)
      fireEvent(video, new Event("playing"))
    })
    expect(screen.queryByText("Click to play the live preview")).not.toBeInTheDocument()

    fireEvent(video, new Event("pause"))
    expect(screen.getByRole("button", { name: "Resume preview" })).toBeInTheDocument()
  })
})
