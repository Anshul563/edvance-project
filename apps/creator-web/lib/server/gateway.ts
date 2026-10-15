import type { ApiError, ApiResponse } from "@/api-client"
import { apiBaseUrl } from "./session"

const GATEWAY_TIMEOUT_MS = 15_000

export interface GatewayResult {
  status: number
  payload: unknown
}

/** Calls the API gateway and returns the parsed status + body (never throws). */
export async function callGateway(
  path: string,
  init?: RequestInit
): Promise<GatewayResult> {
  const controller = new AbortController()
  const timeout = setTimeout(() => controller.abort(), GATEWAY_TIMEOUT_MS)
  try {
    const response = await fetch(`${apiBaseUrl()}${path}`, {
      ...init,
      cache: "no-store",
      signal: controller.signal,
    })
    const text = await response.text().catch(() => "")
    let payload: unknown = null
    if (text) {
      try {
        payload = JSON.parse(text)
      } catch {
        payload = null
      }
    }
    return { status: response.status, payload }
  } catch {
    return { status: 0, payload: null }
  } finally {
    clearTimeout(timeout)
  }
}

export function extractErrorMessage(payload: unknown, fallback: string): string {
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

export function gatewayErrorMessage(result: GatewayResult, fallback: string): string {
  if (result.status === 0) {
    return "Unable to reach the server. Please check your connection and try again."
  }
  return extractErrorMessage(result.payload, fallback)
}

/** Builds an `ApiResponse` JSON response for an auth route handler. */
export function jsonResponse<T>(data: ApiResponse<T>, status = 200): Response {
  return Response.json(data, { status })
}

export function errorResponse(status: number, message: string, dataStatus = status): Response {
  const body: ApiResponse<Record<string, never>> = {
    success: false,
    data: {},
    error: { message, status: dataStatus },
  }
  return Response.json(body, { status })
}

export function failureResponse(result: GatewayResult, fallback: string): Response {
  const status = result.status > 0 ? result.status : 0
  return errorResponse(status > 0 ? status : 502, gatewayErrorMessage(result, fallback), status)
}

export function parseJson<T>(value: unknown): T | null {
  return (value ?? null) as T | null
}

export type { ApiError, ApiResponse }
