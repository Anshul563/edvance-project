import api, { type ApiResponse } from '@/api-client'

// Helper types for course-related API responses
export interface Course {
  id: string
  title: string
  description: string
  level: string
  price: number
  isPublished: boolean
}

export interface CourseLesson {
  id: string
  title: string
  description: string
  order: number
}

export interface CourseSearchFilters {
  q?: string
  category?: string
  level?: string
}

// Course API functions
export async function searchCourses(filters: CourseSearchFilters = {}): Promise<ApiResponse<Course[]>> {
  return api.request<Course[]>('GET', '/api/v1/search', { body: filters })
}

export async function getCourse(courseId: string): Promise<ApiResponse<{
  id: string
  title: string
  description: string
  level: string
  lessons: CourseLesson[]
}>> {
  return api.request(`/api/v1/courses/${courseId}`)
}

// Auth API functions
export async function login(email: string, password: string) {
  return api.login(email, password)
}

export async function logout() {
  return api.logout()
}

// --- Live streaming -------------------------------------------------

export type LiveStreamStatus = 'waiting' | 'live' | 'ended'

export interface LiveStream {
  id: string
  creatorId: string
  title: string
  description?: string
  status: LiveStreamStatus
  statusLabel: string
  streamKeyHint: string
  hasStreamKey: boolean
  rtmpUrl: string
  playbackUrl: string
  createdAt: string
  updatedAt: string
  startedAt?: string
  endedAt?: string
}

export interface CreatedLiveStream extends LiveStream {
  /** Raw publisher key — only present right after create/regenerate. */
  streamKey: string
}

export async function createLiveStream(input: {
  title: string
  description?: string
}): Promise<ApiResponse<CreatedLiveStream>> {
  return api.request<CreatedLiveStream>('POST', '/api/v1/live/streams', { body: input })
}

export async function listMyLiveStreams(): Promise<ApiResponse<{ items: LiveStream[] }>> {
  return api.request<{ items: LiveStream[] }>('GET', '/api/v1/live/streams/mine')
}

export async function getLiveStream(id: string): Promise<ApiResponse<LiveStream>> {
  return api.request<LiveStream>('GET', `/api/v1/live/streams/${id}`)
}

export async function endLiveStream(id: string): Promise<ApiResponse<LiveStream>> {
  return api.request<LiveStream>('POST', `/api/v1/live/streams/${id}/end`)
}

export async function regenerateLiveStreamKey(id: string): Promise<ApiResponse<CreatedLiveStream>> {
  return api.request<CreatedLiveStream>('POST', `/api/v1/live/streams/${id}/regenerate-key`)
}

// Export the api instance for direct use
export { api }
