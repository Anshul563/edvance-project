import { fireEvent, render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { beforeEach, describe, expect, it, vi } from "vitest"

const mocks = vi.hoisted(() => ({
  push: vi.fn(),
  success: vi.fn(),
  error: vi.fn(),
  warning: vi.fn(),
  createVideo: vi.fn(),
  updateVideo: vi.fn(),
  initiateUpload: vi.fn(),
  completeUpload: vi.fn(),
  deleteMediaAsset: vi.fn(),
  listCategories: vi.fn(),
  uploadToStorage: vi.fn(),
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

vi.mock("./video-api", async () => {
  const actual = await vi.importActual<typeof import("./video-api")>("./video-api")
  return {
    ...actual,
    createVideo: mocks.createVideo,
    updateVideo: mocks.updateVideo,
    initiateUpload: mocks.initiateUpload,
    completeUpload: mocks.completeUpload,
    deleteMediaAsset: mocks.deleteMediaAsset,
    listCategories: mocks.listCategories,
    uploadToStorage: mocks.uploadToStorage,
  }
})

import { UploadWizard } from "./upload-wizard"

function videoFile(type = "video/mp4", name = "lesson.mp4") {
  return new File(["video-bytes"], name, { type })
}

function chooseFile(container: HTMLElement, file: File) {
  const input = container.querySelector('input[type="file"]')
  if (!input) throw new Error("file input missing")
  fireEvent.change(input, { target: { files: [file] } })
}

beforeEach(() => {
  vi.clearAllMocks()
  mocks.listCategories.mockResolvedValue({ success: true, data: { items: [] } })
  mocks.createVideo.mockResolvedValue({
    success: true,
    data: { id: "vid_1", title: "Intro", status: "upload_pending" },
  })
  mocks.updateVideo.mockResolvedValue({ success: true, data: { id: "vid_1" } })
  mocks.initiateUpload.mockResolvedValue({
    success: true,
    data: {
      uploadUrl: "https://r2.example/presigned",
      contentType: "video/mp4",
      asset: { id: "ma_1", status: "upload_pending" },
    },
  })
  mocks.completeUpload.mockResolvedValue({
    success: true,
    data: { id: "ma_1", status: "processing" },
  })
  mocks.uploadToStorage.mockResolvedValue(undefined)
})

describe("UploadWizard", () => {
  it("rejects unsupported file types before continuing", async () => {
    const { container } = render(<UploadWizard />)

    chooseFile(container, videoFile("text/plain", "notes.txt"))

    expect(
      await screen.findByText("Choose an MP4, WebM, MOV, or MKV video.")
    ).toBeInTheDocument()
    expect(screen.getByRole("button", { name: "Continue" })).toBeDisabled()
  })

  it("requires metadata and both upload attestations", async () => {
    const user = userEvent.setup()
    const { container } = render(<UploadWizard />)

    chooseFile(container, videoFile())
    await user.click(screen.getByRole("button", { name: "Continue" }))
    expect(
      await screen.findByRole("heading", { name: "Video details" })
    ).toBeInTheDocument()

    await user.click(screen.getByRole("button", { name: "Continue" }))
    expect(
      await screen.findByText("Add a title of at least 3 characters.")
    ).toBeInTheDocument()

    await user.type(
      screen.getByLabelText(/Title/),
      "Introduction to linear algebra"
    )
    await user.click(screen.getByRole("button", { name: "Continue" }))
    expect(
      await screen.findByText(
        "Confirm that the video is educational and that you hold the rights to upload it."
      )
    ).toBeInTheDocument()
  })

  it("creates, links, uploads, and confirms the video before navigating", async () => {
    const user = userEvent.setup()
    const { container } = render(<UploadWizard />)

    chooseFile(container, videoFile())
    await user.click(screen.getByRole("button", { name: "Continue" }))
    await screen.findByRole("heading", { name: "Video details" })

    await user.type(
      screen.getByLabelText(/Title/),
      "Introduction to linear algebra"
    )
    const [educational, rights] = screen.getAllByRole("checkbox")
    await user.click(educational)
    await user.click(rights)
    await user.click(screen.getByRole("button", { name: "Continue" }))

    expect(await screen.findByText("Upload to storage")).toBeInTheDocument()
    await user.click(screen.getByRole("button", { name: "Start upload" }))

    await waitFor(() =>
      expect(mocks.createVideo).toHaveBeenCalledWith(
        expect.objectContaining({
          title: "Introduction to linear algebra",
          visibility: "private",
          language: "en",
          tags: [],
        })
      )
    )
    expect(mocks.initiateUpload).toHaveBeenCalledWith(
      expect.objectContaining({
        educationalConfirmed: true,
        rightsConfirmed: true,
      })
    )
    expect(mocks.updateVideo).toHaveBeenCalledWith("vid_1", {
      mediaAssetId: "ma_1",
    })
    expect(mocks.uploadToStorage).toHaveBeenCalledWith(
      "https://r2.example/presigned",
      expect.any(File),
      "video/mp4",
      expect.objectContaining({ onProgress: expect.any(Function) })
    )
    expect(mocks.completeUpload).toHaveBeenCalledWith("ma_1")
    await waitFor(() =>
      expect(mocks.push).toHaveBeenCalledWith("/creator/videos/vid_1?uploaded=1")
    )
  })

  it("stops before uploading when the metadata draft cannot be created", async () => {
    mocks.createVideo.mockResolvedValue({
      success: false,
      error: { code: "invalid", message: "Title already exists", status: 409 },
    })
    const user = userEvent.setup()
    const { container } = render(<UploadWizard />)

    chooseFile(container, videoFile())
    await user.click(screen.getByRole("button", { name: "Continue" }))
    await user.type(screen.getByLabelText(/Title/), "Duplicate lesson")
    const [educational, rights] = screen.getAllByRole("checkbox")
    await user.click(educational)
    await user.click(rights)
    await user.click(screen.getByRole("button", { name: "Continue" }))

    await user.click(await screen.findByRole("button", { name: "Start upload" }))
    expect(await screen.findByText("Title already exists")).toBeInTheDocument()
    expect(mocks.initiateUpload).not.toHaveBeenCalled()
    expect(mocks.uploadToStorage).not.toHaveBeenCalled()
    expect(mocks.push).not.toHaveBeenCalled()
  })
})
