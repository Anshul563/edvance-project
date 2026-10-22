import { fireEvent, render, screen, waitFor } from "@testing-library/react"
import { beforeEach, describe, expect, it, vi } from "vitest"
import type { Video } from "./video-types"

const mocks = vi.hoisted(() => ({
  push: vi.fn(),
  success: vi.fn(),
  error: vi.fn(),
  getVideo: vi.fn(),
  listCategories: vi.fn(),
  updateVideo: vi.fn(),
  toDataUrl: vi.fn(),
}))

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: mocks.push }),
}))

vi.mock("sonner", () => ({
  toast: {
    success: mocks.success,
    error: mocks.error,
  },
}))

vi.mock("@/lib/image", () => ({
  imageFileToDataUrl: mocks.toDataUrl,
}))

vi.mock("./video-api", () => ({
  getVideo: mocks.getVideo,
  listCategories: mocks.listCategories,
  updateVideo: mocks.updateVideo,
}))

import { VideoEditForm } from "./video-edit-form"

function makeVideo(overrides: Partial<Video> = {}): Video {
  return {
    id: "vid_1",
    creatorId: "cr_1",
    title: "Quadratic equations",
    description: "Solving quadratics.",
    slug: "quadratic-equations",
    language: "en",
    categoryId: "cat_1",
    visibility: "private",
    status: "ready",
    moderationStatus: "approved",
    mediaAssetId: "ma_1",
    durationSeconds: 600,
    viewCount: 0,
    likeCount: 0,
    commentCount: 0,
    createdAt: "2026-10-01T10:00:00Z",
    updatedAt: "2026-10-02T10:00:00Z",
    tags: [{ id: "t_1", name: "algebra", slug: "algebra" }],
    ...overrides,
  }
}

function imageFile() {
  return new File(["image-bytes"], "cover.jpg", { type: "image/jpeg" })
}

beforeEach(() => {
  vi.clearAllMocks()
  mocks.listCategories.mockResolvedValue({
    success: true,
    data: { items: [{ id: "cat_1", name: "Mathematics", slug: "mathematics" }] },
  })
  mocks.updateVideo.mockResolvedValue({ success: true, data: makeVideo() })
  mocks.toDataUrl.mockResolvedValue("data:image/jpeg;base64,AAAA")
})

describe("VideoEditForm thumbnail", () => {
  it("uploads an image and saves it as the thumbnail", async () => {
    mocks.getVideo.mockResolvedValue({ success: true, data: makeVideo() })
    const { container } = render(<VideoEditForm videoId="vid_1" />)

    await screen.findByDisplayValue("Quadratic equations")

    const input = container.querySelector('input[type="file"]')
    if (!input) throw new Error("thumbnail file input missing")
    fireEvent.change(input, { target: { files: [imageFile()] } })

    const preview = await screen.findByAltText("Video thumbnail preview")
    expect(preview).toHaveAttribute("src", "data:image/jpeg;base64,AAAA")
    expect(mocks.toDataUrl).toHaveBeenCalledWith(
      expect.any(File),
      expect.objectContaining({ width: 1280, aspect: 16 / 9 })
    )

    fireEvent.click(screen.getByRole("button", { name: /save changes/i }))

    await waitFor(() => expect(mocks.updateVideo).toHaveBeenCalledTimes(1))
    expect(mocks.updateVideo).toHaveBeenCalledWith(
      "vid_1",
      expect.objectContaining({ thumbnailUrl: "data:image/jpeg;base64,AAAA" })
    )
  })

  it("clears an existing thumbnail when removed", async () => {
    mocks.getVideo.mockResolvedValue({
      success: true,
      data: makeVideo({ thumbnailUrl: "https://cdn.example/old.jpg" }),
    })
    render(<VideoEditForm videoId="vid_1" />)

    await screen.findByDisplayValue("Quadratic equations")
    expect(
      screen.getByAltText("Video thumbnail preview")
    ).toHaveAttribute("src", "https://cdn.example/old.jpg")

    fireEvent.click(screen.getByRole("button", { name: /remove/i }))
    expect(screen.queryByAltText("Video thumbnail preview")).toBeNull()

    fireEvent.click(screen.getByRole("button", { name: /save changes/i }))

    await waitFor(() => expect(mocks.updateVideo).toHaveBeenCalledTimes(1))
    expect(mocks.updateVideo).toHaveBeenCalledWith(
      "vid_1",
      expect.objectContaining({ thumbnailUrl: "" })
    )
  })
})
