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
  const email = typeof record.email === "string" ? record.email.trim() : ""
  if (!email) {
    return errorResponse(400, "Email is required.")
  }

  const result = await callGateway("/api/v1/auth/resend-verification", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ email }),
  })

  if (result.status !== 200) {
    return failureResponse(result, "Unable to resend the verification email right now.")
  }

  const payload = result.payload as { message?: string } | null
  return Response.json({
    success: true,
    data: {
      message:
        payload?.message || "If an account exists for this email, a verification link has been sent.",
    },
  } satisfies ApiResponse<{ message: string }>)
}
