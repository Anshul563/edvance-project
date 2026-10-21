export interface ApiError {
  message: string
  status: number
}

export interface ApiResponse<T> {
  success: boolean
  data: T
  error?: ApiError
}

export type Method = 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE'

export interface RequestOptions {
  headers?: Record<string, string>
  body?: unknown
  timeout?: number
  /** Aborts the request; used to drop stale list/filter responses. */
  signal?: AbortSignal
}
