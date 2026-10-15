import { NextResponse, type NextRequest } from "next/server"
import { callGateway } from "@/lib/server/gateway"
import {
  ACCESS_TOKEN_COOKIE,
  REFRESH_TOKEN_COOKIE,
  apiBaseUrl,
  applySessionTokens,
  clearSessionTokens,
} from "@/lib/server/session"

const FORWARD_TIMEOUT_MS = 20_000

interface RefreshPayload {
  accessToken?: string
  refreshToken?: string
  expiresIn?: number
  user?: unknown
}

interface RotatedSession {
  accessToken: string
  refreshToken: string
  expiresIn?: unknown
  user?: unknown
}

type RouteContext = { params: Promise<{ path: string[] }> }

/**
 * Same-origin gateway proxy: browser -> Next -> API gateway. Keeps HttpOnly
 * tokens off the client, attaches `Authorization`, and transparently rotates
 * an expired access token via the refresh cookie (one retry).
 */
async function handle(request: NextRequest, context: RouteContext, method: string) {
  const { path } = await context.params

  for (const segment of path) {
    if (
      !segment ||
      segment === "." ||
      segment === ".." ||
      segment.includes("/") ||
      segment.includes("\\") ||
      segment.includes("\0")
    ) {
      return Response.json(
        { success: false, data: {}, error: { message: "Invalid request path.", status: 400 } },
        { status: 400 }
      )
    }
  }

  const encodedPath = path.map(encodeURIComponent).join("/")
  const search = request.nextUrl.search
  const target = `${apiBaseUrl()}/${encodedPath}${search}`

  let accessToken = request.cookies.get(ACCESS_TOKEN_COOKIE)?.value
  const refreshToken = request.cookies.get(REFRESH_TOKEN_COOKIE)?.value

  const hasBody = method !== "GET"
  const body = hasBody ? await request.arrayBuffer() : undefined
  const contentType = request.headers.get("content-type")

  const controller = new AbortController()
  const timeout = setTimeout(() => controller.abort(), FORWARD_TIMEOUT_MS)

  const forward = async (token: string | undefined): Promise<Response> => {
    const headers: Record<string, string> = {}
    if (contentType) headers["content-type"] = contentType
    if (token) headers["authorization"] = `Bearer ${token}`
    return fetch(target, {
      method,
      headers,
      ...(body !== undefined ? { body } : {}),
      cache: "no-store",
      redirect: "manual",
      signal: controller.signal,
    })
  }

  let upstream: Response
  try {
    upstream = await forward(accessToken)
  } catch {
    clearTimeout(timeout)
    return Response.json(
      {
        success: false,
        data: {},
        error: {
          message: "Unable to reach the server. Please check your connection and try again.",
          status: 0,
        },
      },
      { status: 502 }
    )
  }
  clearTimeout(timeout)

  let rotated: RotatedSession | null = null
  let staleSession = false

  if (upstream.status === 401 && refreshToken) {
    const refreshed = await callGateway("/api/v1/auth/refresh", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ refreshToken }),
    })
    if (refreshed.status === 200) {
      const payload = refreshed.payload as RefreshPayload | null
      if (payload?.accessToken && payload.refreshToken) {
        rotated = {
          accessToken: payload.accessToken,
          refreshToken: payload.refreshToken,
          expiresIn: payload.expiresIn,
          user: payload.user,
        }
        accessToken = payload.accessToken
        const retryController = new AbortController()
        const retryTimeout = setTimeout(() => retryController.abort(), FORWARD_TIMEOUT_MS)
        try {
          upstream = await fetch(target, {
            method,
            headers: {
              ...(contentType ? { "content-type": contentType } : {}),
              authorization: `Bearer ${accessToken}`,
            },
            ...(body !== undefined ? { body } : {}),
            cache: "no-store",
            redirect: "manual",
            signal: retryController.signal,
          })
        } catch {
          // Keep the rotated tokens; the caller still receives an error body.
        } finally {
          clearTimeout(retryTimeout)
        }
      }
    } else if (refreshed.status === 400 || refreshed.status === 401 || refreshed.status === 403) {
      staleSession = true
    }
  }

  const headers = new Headers()
  const contentTypeHeader = upstream.headers.get("content-type")
  if (contentTypeHeader) headers.set("content-type", contentTypeHeader)
  const cacheControl = upstream.headers.get("cache-control")
  if (cacheControl) headers.set("cache-control", cacheControl)

  const response = new NextResponse(upstream.body, {
    status: upstream.status,
    statusText: upstream.statusText,
    headers,
  })

  if (rotated) {
    applySessionTokens(response, {
      accessToken: rotated.accessToken,
      refreshToken: rotated.refreshToken,
      expiresIn: rotated.expiresIn,
      user: rotated.user,
    })
  }
  if (staleSession) {
    clearSessionTokens(response)
  }

  return response
}

export function GET(request: NextRequest, context: RouteContext) {
  return handle(request, context, "GET")
}

export function POST(request: NextRequest, context: RouteContext) {
  return handle(request, context, "POST")
}

export function PUT(request: NextRequest, context: RouteContext) {
  return handle(request, context, "PUT")
}

export function PATCH(request: NextRequest, context: RouteContext) {
  return handle(request, context, "PATCH")
}

export function DELETE(request: NextRequest, context: RouteContext) {
  return handle(request, context, "DELETE")
}
