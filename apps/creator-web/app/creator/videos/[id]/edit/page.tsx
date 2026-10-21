"use client"

import { useParams } from "next/navigation"
import { VideoEditForm } from "@/components/videos/video-edit-form"

export default function EditVideoPage() {
  const params = useParams<{ id: string }>()
  return <VideoEditForm videoId={params.id} />
}
