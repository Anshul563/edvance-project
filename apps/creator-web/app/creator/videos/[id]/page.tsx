"use client"

import { useParams } from "next/navigation"
import { VideoDetail } from "@/components/videos/video-detail"

export default function VideoDetailPage() {
  const params = useParams<{ id: string }>()
  return <VideoDetail videoId={params.id} />
}
