import { beforeEach, describe, expect, it, vi } from "vitest"

const mocks = vi.hoisted(() => ({
  request: vi.fn(),
}))

vi.mock("@/api-client", () => ({
  default: { request: mocks.request },
}))

import {
  completeUpload,
  deleteMediaAsset,
  getVideo,
  listVideos,
  publishVideo,
  retryProcessing,
  updateVideo,
} from "./video-api"

beforeEach(() => {
  vi.clearAllMocks()
  mocks.request.mockResolvedValue({ success: true, data: {} })
})

describe("video-api", () => {
  it("serializes only provided list filters", async () => {
    await listVideos({
      creatorId: "cr_1",
      page: 2,
      limit: 10,
      status: "ready",
      q: "linear algebra",
      sort: "newest",
      category: "",
    })

    expect(mocks.request).toHaveBeenCalledWith(
      "GET",
      "/api/v1/content/videos?page=2&limit=10&creator_id=cr_1&status=ready&q=linear+algebra&sort=newest",
      { signal: undefined }
    )
  })

  it("forwards abort signals to detail and playback-scoped requests", async () => {
    const controller = new AbortController()
    await getVideo("vid_1", { signal: controller.signal })
    expect(mocks.request).toHaveBeenCalledWith(
      "GET",
      "/api/v1/content/videos/vid_1",
      { signal: controller.signal }
    )
  })

  it("uses PATCH for metadata edits", async () => {
    await updateVideo("vid_1", { title: "Updated", tags: ["algebra"] })
    expect(mocks.request).toHaveBeenCalledWith(
      "PATCH",
      "/api/v1/content/videos/vid_1",
      { body: { title: "Updated", tags: ["algebra"] } }
    )
  })

  it("routes publish, retry, and media cleanup to the right services", async () => {
    await publishVideo("vid_1")
    await retryProcessing("ma_1")
    await completeUpload("ma_1")
    await deleteMediaAsset("ma_1")

    expect(mocks.request).toHaveBeenNthCalledWith(
      1,
      "POST",
      "/api/v1/content/videos/vid_1/publish"
    )
    expect(mocks.request).toHaveBeenNthCalledWith(
      2,
      "POST",
      "/api/v1/videos/ma_1/retry-processing"
    )
    expect(mocks.request).toHaveBeenNthCalledWith(
      3,
      "POST",
      "/api/v1/videos/ma_1/complete"
    )
    expect(mocks.request).toHaveBeenNthCalledWith(
      4,
      "DELETE",
      "/api/v1/videos/ma_1"
    )
  })
})
