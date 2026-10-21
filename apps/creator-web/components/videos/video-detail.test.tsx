import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { beforeEach, describe, expect, it, vi } from "vitest"
import type { Video } from "./video-types"

const mocks = vi.hoisted(() => ({
  push: vi.fn(),
  success: vi.fn(),
  error: vi.fn(),
  warning: vi.fn(),
  getVideo: vi.fn(),
  getPlayback: vi.fn(),
  listCategories: vi.fn(),
  publishVideo: vi.fn(),
  unpublishVideo: vi.fn(),
  retryProcessing: vi.fn(),
  deleteVideo: vi.fn(),
  deleteMediaAsset: vi.fn(),
}))

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: mocks.push }),
}))

vi.mock("sonner", () => ({
  toast: {
    success: mocks.success,
    error: mocks.error,
    warning: mocks.warning,
  },
}))

vi.mock("./video-api", () => ({
  getVideo: mocks.getVideo,
  getPlayback: mocks.getPlayback,
  listCategories: mocks.listCategories,
  publishVideo: mocks.publishVideo,
  unpublishVideo: mocks.unpublishVideo,
  retryProcessing: mocks.retryProcessing,
  deleteVideo: mocks.deleteVideo,
  deleteMediaAsset: mocks.deleteMediaAsset,
}))

import { VideoDetail } from "./video-detail"

function makeVideo(overrides: Partial<Video> = {}): Video {
  return {
    id: "vid_1",
    creatorId: "cr_1",
    title: "Quadratic equations",
    slug: "quadratic-equations",
    language: "en",
    categoryId: "cat_1",
    visibility: "private",
    status: "ready",
    moderationStatus: "approved",
    mediaAssetId: "ma_1",
    durationSeconds: 600,
    viewCount: 12,
    likeCount: 2,
    commentCount: 0,
    createdAt: "2026-10-01T10:00:00Z",
    updatedAt: "2026-10-02T10:00:00Z",
    tags: [{ id: "t_1", name: "algebra", slug: "algebra" }],
    ...overrides,
  }
}

beforeEach(() => {
  vi.clearAllMocks()
  mocks.listCategories.mockResolvedValue({
    success: true,
    data: { items: [{ id: "cat_1", name: "Mathematics", slug: "mathematics" }] },
  })
  mocks.getPlayback.mockResolvedValue({
    success: true,
    data: {
      mediaAssetId: "ma_1",
      status: "ready",
      playbackUrl: "https://cdn.example/vid_1.mp4",
    },
  })
})

describe("VideoDetail", () => {
  it("loads playback for a ready video and shows its metadata", async () => {
    mocks.getVideo.mockResolvedValue({ success: true, data: makeVideo() })
    render(<VideoDetail videoId="vid_1" />)

    expect(
      await screen.findByRole("heading", { name: "Quadratic equations" })
    ).toBeInTheDocument()
    expect(screen.getByText("Mathematics")).toBeInTheDocument()
    expect(screen.getByText("algebra")).toBeInTheDocument()
    await waitFor(() => expect(mocks.getPlayback).toHaveBeenCalledWith("vid_1", expect.anything()))
  })

  it("publishes a ready approved video", async () => {
    mocks.getVideo.mockResolvedValue({ success: true, data: makeVideo() })
    mocks.publishVideo.mockResolvedValue({
      success: true,
      data: makeVideo({ status: "published", visibility: "public" }),
    })
    const user = userEvent.setup()
    render(<VideoDetail videoId="vid_1" />)

    await user.click(await screen.findByRole("button", { name: "Publish" }))
    expect(mocks.publishVideo).toHaveBeenCalledWith("vid_1")
    await waitFor(() => expect(mocks.success).toHaveBeenCalledWith("Video published."))
    expect(
      await screen.findByRole("heading", { name: "Quadratic equations" })
    ).toBeInTheDocument()
  })

  it("shows the publish gate error returned by content-service", async () => {
    mocks.getVideo.mockResolvedValue({ success: true, data: makeVideo() })
    mocks.publishVideo.mockResolvedValue({
      success: false,
      error: {
        code: "moderation_required",
        message: "Video must be approved by moderation before publishing",
        status: 409,
      },
    })
    const user = userEvent.setup()
    render(<VideoDetail videoId="vid_1" />)

    await user.click(await screen.findByRole("button", { name: "Publish" }))
    expect(
      await screen.findByText(
        "Video must be approved by moderation before publishing"
      )
    ).toBeInTheDocument()
    expect(mocks.success).not.toHaveBeenCalled()
  })

  it("offers retry for failed processing and shows the error code", async () => {
    mocks.getVideo.mockResolvedValue({
      success: true,
      data: makeVideo({
        status: "failed",
        moderationStatus: "pending_review",
        processingErrorCode: "ffmpeg_failed",
        processingErrorMessage: "Transcode failed",
      }),
    })
    mocks.retryProcessing.mockResolvedValue({ success: true, data: { id: "ma_1" } })
    const user = userEvent.setup()
    render(<VideoDetail videoId="vid_1" />)

    expect((await screen.findAllByText(/Transcode failed/)).length).toBeGreaterThan(0)
    expect(screen.getAllByText(/ffmpeg_failed/).length).toBeGreaterThan(0)
    expect(screen.queryByRole("button", { name: "Publish" })).not.toBeInTheDocument()

    await user.click(screen.getByRole("button", { name: /Retry/ }))
    expect(mocks.retryProcessing).toHaveBeenCalledWith("ma_1")
    await waitFor(() =>
      expect(mocks.success).toHaveBeenCalledWith("Processing restarted.")
    )
  })
})
