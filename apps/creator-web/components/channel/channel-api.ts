import api, { type ApiResponse } from "@/api-client"

export interface CreatorChannel {
  id: string
  handle: string
  name: string
  description?: string
  bannerUrl?: string
  avatarUrl?: string
}

export interface CreatorProfile {
  id: string
  userId: string
  status: string
  displayName: string
  headline?: string
  bio?: string
  websiteUrl?: string
  coverUrl?: string
  channel: CreatorChannel
  createdAt: string
  updatedAt: string
}

export interface UpdateChannelInput {
  channelName?: string
  description?: string
  avatarUrl?: string
  bannerUrl?: string
}

export function getMyChannel(): Promise<ApiResponse<CreatorProfile>> {
  return api.request<CreatorProfile>("GET", "/api/v1/creators/me")
}

export function updateMyChannel(
  input: UpdateChannelInput
): Promise<ApiResponse<CreatorProfile>> {
  return api.request<CreatorProfile>("PATCH", "/api/v1/creators/me", {
    body: input,
  })
}
