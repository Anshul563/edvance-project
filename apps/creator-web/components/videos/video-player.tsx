"use client"

import { useEffect, useRef } from "react"
import Hls from "hls.js"
import { cn } from "@/lib/utils"

interface VideoPlayerProps {
  playbackUrl?: string
  poster?: string
  className?: string
}

function isHlsUrl(url: string): boolean {
  try {
    return new URL(url, "http://localhost").pathname.toLowerCase().endsWith(".m3u8")
  } catch {
    return url.toLowerCase().includes(".m3u8")
  }
}

/**
 * Player for processed media. The engine may hand back progressive MP4
 * renditions or an HLS manifest; HLS is upgraded with hls.js where the
 * browser lacks native support.
 */
export function VideoPlayer({ playbackUrl, poster, className }: VideoPlayerProps) {
  const videoRef = useRef<HTMLVideoElement>(null)

  useEffect(() => {
    const video = videoRef.current
    if (!video || !playbackUrl) return

    if (!isHlsUrl(playbackUrl)) {
      video.src = playbackUrl
      return () => {
        video.removeAttribute("src")
        video.load()
      }
    }

    if (video.canPlayType("application/vnd.apple.mpegurl")) {
      video.src = playbackUrl
      return () => {
        video.removeAttribute("src")
        video.load()
      }
    }

    if (!Hls.isSupported()) return

    const hls = new Hls()
    hls.loadSource(playbackUrl)
    hls.attachMedia(video)

    return () => {
      hls.destroy()
    }
  }, [playbackUrl])

  if (!playbackUrl) {
    return (
      <div
        className={cn(
          "flex aspect-video w-full items-center justify-center border border-border bg-muted/40 text-xs text-muted-foreground",
          className
        )}
      >
        Preview is not available for this video.
      </div>
    )
  }

  return (
    <video
      ref={videoRef}
      controls
      preload="metadata"
      poster={poster}
      playsInline
      className={cn("aspect-video w-full border border-border bg-black", className)}
    />
  )
}
