import type { NextRequest } from "next/server"
import { callGateway, errorResponse, failureResponse } from "@/lib/server/gateway"
import type { ApiResponse } from "@/api-client"

export async function POST(request: NextRequest) {
  let body: unknown
  try {
    body = await request.json()
  } catch {
    return errorResponse(400, "Invalid request body.")
  }

  const record = (body ?? {}) as Record<string, unknown>
  const token = typeof record.token === "string" ? record.token.trim() : ""
  const newPassword = typeof record.newPassword === "string" ? record.newPassword : ""

  if (!token) {
    return errorResponse(400, "This password reset link is invalid or has expired.")
  }
  if (!newPassword) {
    return errorResponse(400, "New password is required.")
  }

  const result = await callGateway("/api/v1/auth/reset-password", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ token, newPassword }),
  })

  if (result.status !== 200) {
    return failureResponse(result, "Unable to reset your password right now.")
  }

  const payload = result.payload as { message?: string } | null
  return Response.json({
    success: true,
    data: { message: payload?.message || "Your password has been updated." },
  } satisfies ApiResponse<{ message: string }>)
}
