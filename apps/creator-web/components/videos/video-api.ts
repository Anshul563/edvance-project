import api, { type ApiResponse } from "@/api-client"
import type {
  CreatorProfile,
  CreateVideoInput,
  ListVideosParams,
  MediaAsset,
  UpdateVideoInput,
  UploadSession,
  Video,
  VideoCategory,
  VideoPage,
  VideoPlayback,
  VideoVisibility,
} from "./video-types"

const CONTENT_PATH = "/api/v1/content"
const MEDIA_PATH = "/api/v1/videos"

function buildQuery(params: Record<string, string | number | undefined>): string {
  const query = new URLSearchParams()
  for (const [key, value] of Object.entries(params)) {
    if (value === undefined || value === "") continue
    query.set(key, String(value))
  }
  const encoded = query.toString()
  return encoded ? `?${encoded}` : ""
}

// --- Creator profile -------------------------------------------------
// content-service scopes owner listings by the creator profile id, so the
// studio resolves it once and reuses it. Failures are not cached.

let creatorRequest: Promise<ApiResponse<CreatorProfile>> | null = null

export function getMyCreator(): Promise<ApiResponse<CreatorProfile>> {
  if (!creatorRequest) {
    creatorRequest = api
      .request<CreatorProfile>("GET", "/api/v1/creators/me")
      .then((result) => {
        if (!result.success) creatorRequest = null
        return result
      })
  }
  return creatorRequest
}

export function resetCreatorCache(): void {
  creatorRequest = null
}

// --- Metadata --------------------------------------------------------

export function listVideos(
  params: ListVideosParams = {}
): Promise<ApiResponse<VideoPage>> {
  const query = buildQuery({
    page: params.page,
    limit: params.limit,
    creator_id: params.creatorId,
    status: params.status,
    visibility: params.visibility,
    category: params.category,
    language: params.language,
    tag: params.tag,
    q: params.q,
    sort: params.sort,
  })
  return api.request<VideoPage>("GET", `${CONTENT_PATH}/videos${query}`, {
    signal: params.signal,
  })
}

export function getVideo(
  videoId: string,
  options?: { signal?: AbortSignal }
): Promise<ApiResponse<Video>> {
  return api.request<Video>("GET", `${CONTENT_PATH}/videos/${videoId}`, {
    signal: options?.signal,
  })
}

export function createVideo(
  input: CreateVideoInput
): Promise<ApiResponse<Video>> {
  return api.request<Video>("POST", `${CONTENT_PATH}/videos`, { body: input })
}

export function updateVideo(
  videoId: string,
  input: UpdateVideoInput
): Promise<ApiResponse<Video>> {
  return api.request<Video>("PATCH", `${CONTENT_PATH}/videos/${videoId}`, {
    body: input,
  })
}

export function deleteVideo(videoId: string): Promise<ApiResponse<null>> {
  return api.request<null>("DELETE", `${CONTENT_PATH}/videos/${videoId}`)
}

export function publishVideo(videoId: string): Promise<ApiResponse<Video>> {
  return api.request<Video>("POST", `${CONTENT_PATH}/videos/${videoId}/publish`)
}

export function unpublishVideo(videoId: string): Promise<ApiResponse<Video>> {
  return api.request<Video>(
    "POST",
    `${CONTENT_PATH}/videos/${videoId}/unpublish`
  )
}

export function getPlayback(
  videoId: string,
  options?: { signal?: AbortSignal }
): Promise<ApiResponse<VideoPlayback>> {
  return api.request<VideoPlayback>(
    "GET",
    `${CONTENT_PATH}/videos/${videoId}/playback`,
    { signal: options?.signal }
  )
}

export function listCategories(): Promise<ApiResponse<VideoPageCategories>> {
  return api.request<VideoPageCategories>(
    "GET",
    `${CONTENT_PATH}/categories?page=1&limit=100`
  )
}

interface VideoPageCategories {
  items: VideoCategory[]
  page: number
  limit: number
  total: number
  hasNext: boolean
}

// --- Media pipeline (video-service, /api/v1/videos) ------------------

export function initiateUpload(input: {
  file: File
  educationalConfirmed: boolean
  rightsConfirmed: boolean
}): Promise<ApiResponse<UploadSession>> {
  return api.request<UploadSession>("POST", `${MEDIA_PATH}/initiate`, {
    body: {
      filename: input.file.name,
      mimeType: input.file.type,
      size: input.file.size,
      educationalConfirmed: input.educationalConfirmed,
      rightsConfirmed: input.rightsConfirmed,
    },
  })
}

export function completeUpload(
  mediaAssetId: string
): Promise<ApiResponse<MediaAsset>> {
  return api.request<MediaAsset>(
    "POST",
    `${MEDIA_PATH}/${mediaAssetId}/complete`
  )
}

export function retryProcessing(
  mediaAssetId: string
): Promise<ApiResponse<MediaAsset>> {
  return api.request<MediaAsset>(
    "POST",
    `${MEDIA_PATH}/${mediaAssetId}/retry-processing`
  )
}

export function deleteMediaAsset(
  mediaAssetId: string
): Promise<ApiResponse<null>> {
  return api.request<null>("DELETE", `${MEDIA_PATH}/${mediaAssetId}`)
}

export class UploadRequestError extends Error {
  readonly status: number
  readonly expired: boolean

  constructor(message: string, status: number) {
    super(message)
    this.name = "UploadRequestError"
    this.status = status
    this.expired = status === 403 || status === 401
  }
}

/**
 * Uploads directly to object storage with the presigned PUT URL. Bytes
 * never pass through the API gateway and no server credentials touch the
 * browser. XHR is used because it reports upload progress.
 */
export function uploadToStorage(
  uploadUrl: string,
  file: File,
  contentType: string,
  options: {
    onProgress?: (percent: number) => void
    signal?: AbortSignal
  } = {}
): Promise<void> {
  return new Promise((resolve, reject) => {
    const request = new XMLHttpRequest()
    let settled = false

    const finish = (fn: () => void) => {
      if (settled) return
      settled = true
      options.signal?.removeEventListener("abort", handleAbort)
      fn()
    }

    function handleAbort() {
      request.abort()
    }

    request.open("PUT", uploadUrl, true)
    request.setRequestHeader("Content-Type", contentType)

    request.upload.onprogress = (event) => {
      if (!event.lengthComputable) return
      const percent = Math.min(100, Math.round((event.loaded / event.total) * 100))
      options.onProgress?.(percent)
    }

    request.onload = () => {
      if (request.status >= 200 && request.status < 300) {
        options.onProgress?.(100)
        finish(resolve)
        return
      }
      const expired = request.status === 403 || request.status === 401
      finish(() =>
        reject(
          new UploadRequestError(
            expired
              ? "The upload link expired before the file finished uploading."
              : "The storage provider rejected the upload.",
            request.status
          )
        )
      )
    }

    request.onerror = () => {
      finish(() =>
        reject(
          new UploadRequestError(
            "The upload could not reach object storage. Check your connection and try again.",
            0
          )
        )
      )
    }

    request.onabort = () => {
      finish(() =>
        reject(new UploadRequestError("Upload cancelled.", 0))
      )
    }

    if (options.signal?.aborted) {
      finish(() => reject(new UploadRequestError("Upload cancelled.", 0)))
      return
    }

    options.signal?.addEventListener("abort", handleAbort, { once: true })
    request.send(file)
  })
}

export type { VideoVisibility }
