import type { ApiError, ApiResponse, Method, RequestOptions } from './types'

// Development API base URLs - override via environment variables in production
const DEVELOPMENT_BASE_URLS = {
  api: 'http://localhost:8080',
  auth: 'http://localhost:8081',
  user: 'http://localhost:8082',
  creator: 'http://localhost:8083',
  course: 'http://localhost:8086',
  learning: 'http://localhost:8087',
  search: 'http://localhost:8091',
}

// Production base URLs - override via environment variables
const PRODUCTION_BASE_URLS = {
  api: 'https://api.edvance.dev',
  auth: 'https://auth.edvance.dev',
  user: 'https://user.edvance.dev',
  creator: 'https://creator.edvance.dev',
  course: 'https://course.edvance.dev',
  learning: 'https://learning.edvance.dev',
  search: 'https://search.edvance.dev',
}

// Get the current environment and base URL for a service
function getBaseUrl(service: keyof typeof DEVELOPMENT_BASE_URLS): string {
  const isProduction = process.env.NODE_ENV === 'production'
  const urls = isProduction ? PRODUCTION_BASE_URLS : DEVELOPMENT_BASE_URLS
  return urls[service as keyof typeof urls] || DEVELOPMENT_BASE_URLS.api
}

/**
 * Core fetch wrapper with error handling and timeout.
 * Works in both browser/Next.js and React Native environments.
 */
async function apiFetch(
  input: RequestInfo,
  init: RequestInit = {},
  service: keyof typeof DEVELOPMENT_BASE_URLS = 'api' as keyof typeof DEVELOPMENT_BASE_URLS
): Promise<Response> {
  // If input is already a Request, use it directly
  if (input instanceof Request) {
    return fetch(input, init)
  }

  // Otherwise, construct the full URL from the service base + path
  const baseUrl = getBaseUrl(service)
  const url = new URL(input as string, baseUrl).toString()

  // Set default timeout if not provided
  const timeout = init.timeout ?? 15000

  const controller = new AbortController()
  const timeoutId = setTimeout(() => controller.abort(), timeout)

  // Ensure headers are initialized
  const initHeaders: Record<string, string> = {}
  if (init.headers) {
    for (const [key, value] of Object.entries(init.headers)) {
      initHeaders[key] = String(value)
    }
  }

  const initWithTimeout: RequestInit = {
    signal: controller.signal,
    headers: {
      'Content-Type': 'application/json',
      ...initHeaders,
    },
  }

  return fetch(url, initWithTimeout)
}

export interface ApiError {
  message: string
  status: number
}

export interface ApiResponse<T> {
  success: boolean
  data: T
  error?: ApiError
}

export type Method = 'GET' | 'POST' | 'PUT' | 'DELETE'

export interface RequestOptions {
  headers?: Record<string, string>
  body?: unknown
  timeout?: number
}

export class ApiClient {
  private token: string | null = null

  // Base URLs for different services
  private baseUrls = {
    auth: getBaseUrl('auth'),
    user: getBaseUrl('user'),
    creator: getBaseUrl('creator'),
    course: getBaseUrl('course'),
    learning: getBaseUrl('learning'),
    search: getBaseUrl('search'),
  }

  // Set authentication token
  setToken(token: string) {
    this.token = token
  }

  // Get authentication headers
  private getAuthHeaders(): HeadersInit {
    if (!this.token) return {}
    return { Authorization: `Bearer ${this.token}` }
  }

  // Generic request method
  async request<T>(
    method: Method,
    path: string,
    options: RequestOptions = {}
  ): Promise<ApiResponse<T>> {
    const baseUrl = this.baseUrls['course'] || DEVELOPMENT_BASE_URLS.course
    const url = new URL(path, baseUrl).toString()

    const init: RequestInit = {
      method,
      headers: {
        ...this.getAuthHeaders(),
        ...options.headers,
      },
      ...(options.body && { body: JSON.stringify(options.body) }),
    }

    try {
      const response = await apiFetch(url, init, 'api')
      
      const data: ApiResponse<T> = await response.json()
      
      if (!response.ok) {
        return {
          success: false,
          data: {} as T,
          error: {
            message: data.error?.message || response.statusText,
            status: response.status,
          },
        }
      }

      return {
        success: true,
        data,
      }
    } catch (err) {
      // Network error or timeout
      return {
        success: false,
        data: {} as T,
        error: {
          message: err instanceof Error ? err.message : 'Network error',
          status: 0,
        },
      }
    }
  }

  // Auth endpoints
  async register(email: string, username: string, password: string, displayName: string) {
    return this.request<{
      id: string
      email: string
      username: string
      displayName: string
      emailVerified: boolean
      status: string
    }>('POST', '/api/v1/auth/register', {
      body: { email, username, password, displayName },
    })
  }

  async login(email: string, password: string) {
    const result = await this.request<{
      token: string
      user: {
        id: string
        email: string
        username: string
        displayName: string
      }
    }>('POST', '/api/v1/auth/login', {
      body: { email, password },
    })

    if (result.success && result.data?.token) {
      this.setToken(result.data.token)
    }

    return result
  }

  async refreshToken(token: string) {
    return this.request<{
      token: string
    }>('POST', '/api/v1/auth/refresh', {
      body: { token },
    })
  }

  async logout() {
    return this.request<void>('POST', '/api/v1/auth/logout', {})
  }

  // User endpoints
  async getUserProfile() {
    return this.request<{
      id: string
      email: string
      username: string
      displayName: string
    }>('GET', '/api/v1/users/me')
  }

  // Course endpoints
  async searchCourses(filters: { q?: string; category?: string; level?: string }) {
    return this.request<{
      courses: Array<{
        id: string
        title: string
        description: string
        level: string
        price: number
        isPublished: boolean
      }>
    }>('GET', '/api/v1/search', {
      body: filters,
    })
  }

  async getCourse(courseId: string) {
    return this.request<{
      id: string
      title: string
      description: string
      level: string
      lessons: Array<{
        id: string
        title: string
        description: string
        order: number
      }>
    }>('GET', `/api/v1/courses/${courseId}`)
  }

  // Creator endpoints
  async createCourse(courseData: {
    title: string
    description: string
    level: string
  }) {
    return this.request<{
      id: string
      title: string
      description: string
    }>('POST', '/api/v1/courses', {
      body: courseData,
    })
  }
}

// Export a singleton instance for convenience
export const api = new ApiClient()
