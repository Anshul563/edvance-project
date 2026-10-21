import { beforeEach, describe, expect, it, vi } from "vitest"

const mocks = vi.hoisted(() => ({
  request: vi.fn(),
}))

vi.mock("@/api-client", () => ({
  default: { request: mocks.request },
}))

import { getMyChannel, updateMyChannel } from "./channel-api"

beforeEach(() => {
  vi.clearAllMocks()
  mocks.request.mockResolvedValue({ success: true, data: {} })
})

describe("channel-api", () => {
  it("loads the signed-in creator's channel", async () => {
    await getMyChannel()

    expect(mocks.request).toHaveBeenCalledWith("GET", "/api/v1/creators/me")
  })

  it("patches only the supplied channel fields", async () => {
    await updateMyChannel({
      channelName: "Ada Learns",
      description: "Deep dives into algebra.",
      avatarUrl: "data:image/jpeg;base64,abc",
    })

    expect(mocks.request).toHaveBeenCalledWith("PATCH", "/api/v1/creators/me", {
      body: {
        channelName: "Ada Learns",
        description: "Deep dives into algebra.",
        avatarUrl: "data:image/jpeg;base64,abc",
      },
    })
  })
})
