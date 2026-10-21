import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { beforeEach, describe, expect, it, vi } from "vitest"
import type { Video } from "./video-types"

const mocks = vi.hoisted(() => ({
  push: vi.fn(),
  success: vi.fn(),
  error: vi.fn(),
  warning: vi.fn(),
  getMyCreator: vi.fn(),
  resetCreatorCache: vi.fn(),
  listVideos: vi.fn(),
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
  getMyCreator: mocks.getMyCreator,
  resetCreatorCache: mocks.resetCreatorCache,
  listVideos: mocks.listVideos,
  listCategories: mocks.listCategories,
  publishVideo: mocks.publishVideo,
  unpublishVideo: mocks.unpublishVideo,
  retryProcessing: mocks.retryProcessing,
  deleteVideo: mocks.deleteVideo,
  deleteMediaAsset: mocks.deleteMediaAsset,
}))

import { VideoLibrary } from "./video-library"

const CREATOR = {
  success: true as const,
  data: { id: "cr_1", userId: "u_1", status: "active", displayName: "Ada" },
}

function makeVideo(overrides: Partial<Video> = {}): Video {
  return {
    id: "vid_1",
    creatorId: "cr_1",
    title: "Linear algebra basics",
    slug: "linear-algebra-basics",
    language: "en",
    visibility: "private",
    status: "ready",
    moderationStatus: "approved",
    mediaAssetId: "ma_1",
    durationSeconds: 125,
    viewCount: 0,
    likeCount: 0,
    commentCount: 0,
    createdAt: "2026-10-01T10:00:00Z",
    updatedAt: "2026-10-02T10:00:00Z",
    tags: [],
    ...overrides,
  }
}

function pageOf(items: Video[], total = items.length) {
  return {
    success: true as const,
    data: { items, page: 1, limit: 10, total, hasNext: false },
  }
}

beforeEach(() => {
  vi.clearAllMocks()
  mocks.getMyCreator.mockResolvedValue(CREATOR)
  mocks.listCategories.mockResolvedValue({ success: true, data: { items: [] } })
  mocks.listVideos.mockResolvedValue(pageOf([]))
  mocks.deleteMediaAsset.mockResolvedValue({ success: true, data: null })
})

describe("VideoLibrary", () => {
  it("shows a creator-profile error and retries the lookup", async () => {
    mocks.getMyCreator.mockResolvedValueOnce({
      success: false,
      error: { code: "no_creator", message: "Creator profile not found", status: 404 },
    })
    const user = userEvent.setup()
    render(<VideoLibrary />)

    expect(await screen.findByText("Creator profile not found")).toBeInTheDocument()
    expect(screen.getByText(/A creator profile is required/)).toBeInTheDocument()

    mocks.getMyCreator.mockResolvedValueOnce(CREATOR)
    await user.click(screen.getByRole("button", { name: "Try again" }))
    await waitFor(() => expect(mocks.listVideos).toHaveBeenCalled())
  })

  it("renders videos with status-aware actions", async () => {
    mocks.listVideos.mockResolvedValue(
      pageOf([
        makeVideo(),
        makeVideo({ id: "vid_2", title: "Still processing", status: "processing" }),
        makeVideo({ id: "vid_3", title: "Live lesson", status: "published" }),
      ])
    )
    render(<VideoLibrary />)

    expect((await screen.findAllByText("Linear algebra basics")).length).toBeGreaterThan(0)
    expect(screen.getAllByText("Still processing").length).toBeGreaterThan(0)
    expect(screen.getAllByText("Live lesson").length).toBeGreaterThan(0)

    expect(screen.getAllByRole("button", { name: "Publish" }).length).toBeGreaterThan(0)
    expect(screen.getAllByRole("button", { name: "Unpublish" }).length).toBeGreaterThan(0)
    expect(screen.queryAllByRole("button", { name: /Retry/ })).toHaveLength(0)
  })

  it("publishes a ready video and refreshes the list", async () => {
    mocks.listVideos.mockResolvedValue(pageOf([makeVideo()]))
    mocks.publishVideo.mockResolvedValue({
      success: true,
      data: makeVideo({ status: "published", visibility: "public" }),
    })
    const user = userEvent.setup()
    render(<VideoLibrary />)

    await user.click((await screen.findAllByRole("button", { name: "Publish" }))[0])
    expect(mocks.publishVideo).toHaveBeenCalledWith("vid_1")
    await waitFor(() => expect(mocks.success).toHaveBeenCalledWith("Video published."))
    expect(mocks.listVideos).toHaveBeenCalledTimes(2)
  })

  it("surfaces publish errors without refreshing", async () => {
    mocks.listVideos.mockResolvedValue(pageOf([makeVideo()]))
    mocks.publishVideo.mockResolvedValue({
      success: false,
      error: { code: "not_approved", message: "Video is still in review", status: 409 },
    })
    const user = userEvent.setup()
    render(<VideoLibrary />)

    await user.click((await screen.findAllByRole("button", { name: "Publish" }))[0])
    await waitFor(() =>
      expect(mocks.error).toHaveBeenCalledWith("Video is still in review")
    )
    expect(mocks.success).not.toHaveBeenCalled()
  })

  it("passes the selected filters to the videos endpoint", async () => {
    const user = userEvent.setup()
    render(<VideoLibrary />)

    await screen.findByText("No videos yet")
    await user.selectOptions(screen.getByLabelText("Status"), "ready")
    await waitFor(() =>
      expect(mocks.listVideos).toHaveBeenLastCalledWith(
        expect.objectContaining({ status: "ready", creatorId: "cr_1", sort: "newest" })
      )
    )
  })

  it("deletes metadata and then the media asset after confirmation", async () => {
    mocks.listVideos.mockResolvedValue(pageOf([makeVideo()]))
    mocks.deleteVideo.mockResolvedValue({ success: true, data: null })
    const user = userEvent.setup()
    render(<VideoLibrary />)

    await user.click(
      (
        await screen.findAllByRole("button", {
          name: "Delete Linear algebra basics",
        })
      )[0]
    )
    expect(screen.getByText("Delete this video?")).toBeInTheDocument()
    await user.click(screen.getByRole("button", { name: "Delete" }))

    await waitFor(() => expect(mocks.deleteVideo).toHaveBeenCalledWith("vid_1"))
    expect(mocks.deleteMediaAsset).toHaveBeenCalledWith("ma_1")
    await waitFor(() => expect(mocks.success).toHaveBeenCalledWith("Video deleted."))
  })

  it("shows the empty state with an upload action", async () => {
    const user = userEvent.setup()
    render(<VideoLibrary />)

    const uploadButtons = await screen.findAllByRole("button", { name: "Upload Video" })
    await user.click(uploadButtons[uploadButtons.length - 1])
    expect(mocks.push).toHaveBeenCalledWith("/creator/videos/upload")
  })
})
