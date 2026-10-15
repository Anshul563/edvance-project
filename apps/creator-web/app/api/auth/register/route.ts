import type { NextRequest } from "next/server"
import { callGateway, errorResponse, failureResponse } from "@/lib/server/gateway"
import type { ApiResponse, RegisterResult } from "@/api-client"

interface RegisterPayload {
  id?: string
  email?: string
  username?: string
  displayName?: string
  emailVerified?: boolean
  status?: string
}

export async function POST(request: NextRequest) {
  let body: unknown
  try {
    body = await request.json()
  } catch {
    return errorResponse(400, "Invalid request body.")
  }

  const record = (body ?? {}) as Record<string, unknown>
  const email = typeof record.email === "string" ? record.email.trim() : ""
  const username = typeof record.username === "string" ? record.username.trim() : ""
  const password = typeof record.password === "string" ? record.password : ""
  const displayName = typeof record.displayName === "string" ? record.displayName.trim() : ""

  if (!email || !username || !password || !displayName) {
    return errorResponse(400, "All fields are required.")
  }

  const result = await callGateway("/api/v1/auth/register", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ email, username, password, displayName }),
  })

  if (result.status !== 201) {
    return failureResponse(result, "Unable to create your account right now.")
  }

  const payload = result.payload as RegisterPayload | null
  if (!payload) {
    return errorResponse(502, "The registration response was incomplete. Please try again.")
  }

  const data: RegisterResult = {
    id: payload.id ?? "",
    email: payload.email ?? email,
    username: payload.username ?? username,
    displayName: payload.displayName ?? displayName,
    emailVerified: payload.emailVerified ?? false,
    status: payload.status ?? "pending",
  }

  return Response.json({ success: true, data } satisfies ApiResponse<RegisterResult>, {
    status: 201,
  })
}
