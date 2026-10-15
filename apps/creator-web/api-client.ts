import type { ApiError, Method, RequestOptions } from "@api-client/types"

export type { ApiError, Method, RequestOptions }

/**
 * Result envelope shared by the Next auth/proxy routes and this client.
 * Modelled as a discriminated union so `!result.success` narrows `error`.
 */
export type ApiResponse<T> =
  | { success: true; data: T }
  | { success: false; data: T; error: ApiError }

export interface AuthUser {
  id: string
  email: string
  username: string
  displayName: string
  emailVerified: boolean
}

export interface SessionUser extends AuthUser {
  status?: string
}

export interface LoginSession {
  user: AuthUser
}

export interface RegisterResult {
  id: string
  email: string
  username: string
  displayName: string
  emailVerified: boolean
  status: string
}

export interface RegisterInput {
  email: string
  username: string
  password: string
  displayName: string
}

const REQUEST_TIMEOUT_MS = 15_000

function apiBase(): string {
  return (
    process.env.AUTH_API_BASE_URL ??
    process.env.NEXT_PUBLIC_API_URL ??
    "http://localhost:8080"
  )
}

function backendUrl(path: string): string {
  const base = typeof window === "undefined" ? apiBase() : ""
  return `${base}/api/backend${path}`
}

async function readBody(response: Response): Promise<unknown> {
  const text = await response.text().catch(() => "")
  if (!text) return null
  try {
    return JSON.parse(text) as unknown
  } catch {
    return null
  }
}

function extractErrorMessage(payload: unknown, fallback: string): string {
  if (payload && typeof payload === "object") {
    const record = payload as Record<string, unknown>
    if (typeof record.error === "string" && record.error.trim()) return record.error
    if (typeof record.message === "string" && record.message.trim()) return record.message
    const nested = record.error
    if (nested && typeof nested === "object" && "message" in nested) {
      const message = (nested as Record<string, unknown>).message
      if (typeof message === "string" && message.trim()) return message
    }
  }
  return fallback
}

async function toApiResponse<T>(response: Response, fallback: string): Promise<ApiResponse<T>> {
  const payload = await readBody(response)
  if (!response.ok) {
    return {
      success: false,
      data: {} as T,
      error: {
        message: extractErrorMessage(payload, fallback),
        status: response.status,
      },
    }
  }
  return {
    success: true,
    data: payload as T,
  }
}

function networkError<T>(err: unknown): ApiResponse<T> {
  const message =
    err instanceof DOMException && err.name === "TimeoutError"
      ? "The request timed out. Please try again."
      : "Unable to reach the server. Please check your connection and try again."
  return {
    success: false,
    data: {} as T,
    error: { message, status: 0 },
  }
}

async function fetchWithTimeout(url: string, init: RequestInit): Promise<Response> {
  const controller = new AbortController()
  const timeout = setTimeout(() => controller.abort(), REQUEST_TIMEOUT_MS)
  try {
    return await fetch(url, { ...init, signal: controller.signal, cache: "no-store" })
  } finally {
    clearTimeout(timeout)
  }
}

async function authRequest<T>(
  path: string,
  method: "GET" | "POST",
  body?: unknown
): Promise<ApiResponse<T>> {
  try {
    const response = await fetchWithTimeout(path, {
      method,
      headers: { "Content-Type": "application/json" },
      ...(body !== undefined ? { body: JSON.stringify(body) } : {}),
    })
    return await toApiResponse<T>(response, "Authentication request failed.")
  } catch (err) {
    return networkError<T>(err)
  }
}

function queryFromBody(body: unknown): string {
  if (!body || typeof body !== "object") return ""
  const params = new URLSearchParams()
  for (const [key, value] of Object.entries(body as Record<string, unknown>)) {
    if (value !== undefined && value !== null) params.set(key, String(value))
  }
  const encoded = params.toString()
  return encoded ? `?${encoded}` : ""
}

export class ApiClient {
  async request<T>(path: string): Promise<ApiResponse<T>>
  async request<T>(method: Method, path: string, options?: RequestOptions): Promise<ApiResponse<T>>
  async request<T>(
    methodOrPath: Method | string,
    pathOrOptions?: string | RequestOptions,
    maybeOptions?: RequestOptions
  ): Promise<ApiResponse<T>> {
    const firstArgIsPath = typeof pathOrOptions !== "string"
    const method = (firstArgIsPath ? "GET" : (methodOrPath as Method)) as Method
    const path = firstArgIsPath ? (methodOrPath as string) : (pathOrOptions as string)
    const options = (firstArgIsPath
      ? (pathOrOptions as RequestOptions | undefined)
      : maybeOptions) ?? {}

    const headers: Record<string, string> = { ...(options.headers ?? {}) }
    let url = backendUrl(path)
    let body: BodyInit | undefined

    if (options.body !== undefined && options.body !== null) {
      if (method === "GET" || method === "DELETE") {
        url += queryFromBody(options.body)
      } else {
        headers["Content-Type"] = "application/json"
        body = JSON.stringify(options.body)
      }
    }

    try {
      const response = await fetchWithTimeout(url, { method, headers, body })
      return await toApiResponse<T>(response, `Request failed (${method} ${path}).`)
    } catch (err) {
      return networkError<T>(err)
    }
  }

  async login(email: string, password: string): Promise<ApiResponse<LoginSession>> {
    return authRequest<LoginSession>("/api/auth/login", "POST", { email, password })
  }

  async register(input: RegisterInput): Promise<ApiResponse<RegisterResult>> {
    return authRequest<RegisterResult>("/api/auth/register", "POST", input)
  }

  async forgotPassword(email: string): Promise<ApiResponse<{ message: string }>> {
    return authRequest<{ message: string }>("/api/auth/forgot-password", "POST", { email })
  }

  async resetPassword(token: string, newPassword: string): Promise<ApiResponse<{ message: string }>> {
    return authRequest<{ message: string }>("/api/auth/reset-password", "POST", {
      token,
      newPassword,
    })
  }

  async verifyEmail(token: string): Promise<ApiResponse<{ message: string }>> {
    return authRequest<{ message: string }>(
      `/api/auth/verify-email?token=${encodeURIComponent(token)}`,
      "GET"
    )
  }

  async resendVerification(email: string): Promise<ApiResponse<{ message: string }>> {
    return authRequest<{ message: string }>("/api/auth/resend-verification", "POST", { email })
  }

  async refreshSession(): Promise<ApiResponse<LoginSession>> {
    return authRequest<LoginSession>("/api/auth/refresh", "POST")
  }

  async logout(): Promise<ApiResponse<{ message: string }>> {
    return authRequest<{ message: string }>("/api/auth/logout", "POST")
  }
}

export const api = new ApiClient()
export default api
