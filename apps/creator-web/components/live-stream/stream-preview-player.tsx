"use client"

import { useCallback, useEffect, useRef, useState } from "react"
import Hls from "hls.js"
import { HugeiconsIcon } from "@hugeicons/react"
import {
  PlayIcon,
  PauseIcon,
  VolumeMuteIcon,
  VolumeUpIcon,
  Maximize01Icon,
  Minimize02Icon,
  Loading03Icon,
  AntennaIcon,
  Cancel01Icon,
  ArrowUpRightIcon,
} from "@hugeicons/core-free-icons"
import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"
import { StreamStatusBadge } from "./stream-status-badge"
import { StreamErrorState } from "./stream-error-state"
import type { LiveStreamStatus } from "@/components/api-client"

/**
 * Every state the preview can visibly be in. Publisher status (from the
 * backend), media availability (HLS manifest/segments) and player status
 * (video element events) are tracked separately and folded into one of
 * these display states.
 */
export type StreamPreviewState =
  | "idle"
  | "waiting-publisher"
  | "waiting-media"
  | "ready"
  | "buffering"
  | "playing"
  | "paused"
  | "interrupted"
  | "ended"
  | "invalid-url"
  | "unsupported"
  | "error"

type MediaState = "waiting-media" | "ready" | "buffering" | "playing" | "paused" | "interrupted" | "error"

const MAX_AUTO_RETRIES = 8
const RETRY_DELAY_MS = 3000

function nativeHlsSupported(): boolean {
  if (typeof document === "undefined") return false
  const probe = document.createElement("video")
  return probe.canPlayType("application/vnd.apple.mpegurl") !== ""
}

interface StreamPreviewPlayerProps {
  playbackUrl: string
  status: LiveStreamStatus
  /** Server-provided status label for the on-video chip. */
  statusLabel?: string
  className?: string
}

export function StreamPreviewPlayer({
  playbackUrl,
  status,
  statusLabel,
  className,
}: StreamPreviewPlayerProps) {
  const videoRef = useRef<HTMLVideoElement | null>(null)
  const containerRef = useRef<HTMLDivElement | null>(null)
  const attemptsRef = useRef(0)
  const recoveredRef = useRef(false)
  const retryTimerRef = useRef<number | null>(null)

  const [engineSupported] = useState(() =>
    typeof document === "undefined" ? true : Hls.isSupported() || nativeHlsSupported(),
  )
  const [mediaState, setMediaState] = useState<MediaState>("waiting-media")
  const [muted, setMuted] = useState(true)
  const [volume, setVolume] = useState(0.8)
  const [isFullscreen, setIsFullscreen] = useState(false)
  const [retryTick, setRetryTick] = useState(0)

  const hasSource = playbackUrl.trim() !== ""

  const effectiveState: StreamPreviewState = (() => {
    if (status === "ended") return "ended"
    if (status === "waiting") return "waiting-publisher"
    if (!hasSource) return "invalid-url"
    if (!engineSupported) return "unsupported"
    if (status !== "live") return "waiting-publisher"
    return mediaState
  })()

  // Attach the HLS engine whenever there is a live stream with a source.
  // State is only mutated from engine/video event callbacks or timers.
  useEffect(() => {
    const video = videoRef.current
    if (!video || !engineSupported || !hasSource || status !== "live") return

    const clearRetry = () => {
      if (retryTimerRef.current !== null) {
        window.clearTimeout(retryTimerRef.current)
        retryTimerRef.current = null
      }
    }
    const scheduleRetry = () => {
      clearRetry()
      retryTimerRef.current = window.setTimeout(
        () => setRetryTick((tick) => tick + 1),
        RETRY_DELAY_MS,
      )
    }

    clearRetry()
    // Attempts are intentionally NOT reset here: each automatic retry
    // re-runs this effect, and the counter must survive re-runs so the
    // preview eventually surfaces "interrupted" instead of looping.
    // Successful loads (MANIFEST_PARSED/FRAG_BUFFERED) and the manual
    // Retry action reset it.
    recoveredRef.current = false

    const onPlaying = () => setMediaState("playing")
    const onWaiting = () => setMediaState((prev) => (prev === "paused" ? prev : "buffering"))
    const onPause = () => setMediaState((prev) => (prev === "playing" ? "paused" : prev))
    const onVolumeChange = () => {
      setMuted(video.muted)
      setVolume(video.volume)
    }
    const onVideoError = () => {
      if (attemptsRef.current < MAX_AUTO_RETRIES) {
        attemptsRef.current += 1
        setMediaState("waiting-media")
        scheduleRetry()
        return
      }
      setMediaState("interrupted")
    }

    video.addEventListener("playing", onPlaying)
    video.addEventListener("waiting", onWaiting)
    video.addEventListener("pause", onPause)
    video.addEventListener("volumechange", onVolumeChange)
    video.addEventListener("error", onVideoError)

    const tryAutoplay = () => {
      video.muted = true
      void Promise.resolve(video.play()).catch(() => {
        // Autoplay blocked by the browser — the overlay invites a click.
      })
    }

    let detachEngine: () => void

    if (Hls.isSupported()) {
      const hls = new Hls({
        enableWorker: true,
        lowLatencyMode: true,
        // Stay a few segments behind the live edge so small encoder
        // hiccups don't cause repeated rebuffering.
        liveSyncDurationCount: 3,
        liveMaxLatencyDurationCount: 12,
        // Retry generously before giving up on the preview.
        manifestLoadingMaxRetry: 6,
        levelLoadingMaxRetry: 6,
        fragLoadingMaxRetry: 6,
        backBufferLength: 60,
      })

      hls.on(Hls.Events.MANIFEST_PARSED, () => {
        attemptsRef.current = 0
        recoveredRef.current = false
        setMediaState("ready")
        tryAutoplay()
      })
      hls.on(Hls.Events.FRAG_BUFFERED, () => {
        attemptsRef.current = 0
        recoveredRef.current = false
      })
      // Canonical hls.js recovery: fatal errors retry with backoff instead
      // of instantly failing the preview. Right after OBS connects the
      // playlist can 404 for a few seconds — that is “waiting for media”,
      // not an error.
      hls.on(Hls.Events.ERROR, (_event, data) => {
        if (!data.fatal) return
        if (data.type === Hls.ErrorTypes.NETWORK_ERROR) {
          if (attemptsRef.current < MAX_AUTO_RETRIES) {
            attemptsRef.current += 1
            setMediaState("waiting-media")
            scheduleRetry()
            return
          }
          setMediaState("interrupted")
          return
        }
        if (data.type === Hls.ErrorTypes.MEDIA_ERROR) {
          if (!recoveredRef.current) {
            recoveredRef.current = true
            setMediaState("buffering")
            hls.recoverMediaError()
            return
          }
          setMediaState("error")
          hls.destroy()
          return
        }
        setMediaState("error")
        hls.destroy()
      })

      hls.loadSource(playbackUrl)
      hls.attachMedia(video)
      detachEngine = () => hls.destroy()
    } else {
      // Native HLS (Safari and iOS browsers).
      video.src = playbackUrl
      const onLoadedMetadata = () => {
        attemptsRef.current = 0
        setMediaState("ready")
        tryAutoplay()
      }
      video.addEventListener("loadedmetadata", onLoadedMetadata)
      detachEngine = () => video.removeEventListener("loadedmetadata", onLoadedMetadata)
    }

    return () => {
      clearRetry()
      video.removeEventListener("playing", onPlaying)
      video.removeEventListener("waiting", onWaiting)
      video.removeEventListener("pause", onPause)
      video.removeEventListener("volumechange", onVolumeChange)
      video.removeEventListener("error", onVideoError)
      detachEngine()
      video.removeAttribute("src")
      video.load()
    }
  }, [engineSupported, hasSource, playbackUrl, status, retryTick])

  useEffect(() => {
    const onFullscreenChange = () =>
      setIsFullscreen(document.fullscreenElement === containerRef.current)
    document.addEventListener("fullscreenchange", onFullscreenChange)
    return () => document.removeEventListener("fullscreenchange", onFullscreenChange)
  }, [])

  const handleRetry = useCallback(() => {
    attemptsRef.current = 0
    setRetryTick((tick) => tick + 1)
  }, [])

  function togglePlay() {
    const video = videoRef.current
    if (!video) return
    if (video.paused) void Promise.resolve(video.play()).catch(() => undefined)
    else video.pause()
  }

  function toggleMute() {
    const video = videoRef.current
    if (video) video.muted = !video.muted
  }

  function handleVolumeInput(next: number) {
    const video = videoRef.current
    setVolume(next)
    if (video) {
      video.volume = next
      if (next > 0 && video.muted) video.muted = false
    }
  }

  function toggleFullscreen() {
    const element = containerRef.current
    if (!element) return
    if (document.fullscreenElement) {
      void document.exitFullscreen().catch(() => undefined)
    } else {
      void element.requestFullscreen?.().catch(() => undefined)
    }
  }

  const isPlaying = effectiveState === "playing"
  const controlsEnabled =
    effectiveState === "playing" ||
    effectiveState === "paused" ||
    effectiveState === "ready" ||
    effectiveState === "buffering"

  return (
    <div
      ref={containerRef}
      className={cn("overflow-hidden border border-border bg-black", className)}
    >
      <div className="relative aspect-video w-full">
        <video
          ref={videoRef}
          playsInline
          onClick={togglePlay}
          className="absolute inset-0 size-full object-contain"
        />

        <div className="pointer-events-none absolute left-3 top-3 z-10">
          <StreamStatusBadge
            status={status}
            statusLabel={statusLabel}
            className="bg-black/60 text-white backdrop-blur border-white/25"
          />
        </div>

        {effectiveState === "playing" ? null : effectiveState === "paused" ? (
          <button
            type="button"
            onClick={togglePlay}
            aria-label="Resume preview"
            className="absolute inset-0 z-10 flex items-center justify-center bg-black/40 transition-colors hover:bg-black/30"
          >
            <span className="flex size-14 items-center justify-center rounded-full bg-white/90 text-black">
              <HugeiconsIcon icon={PlayIcon} strokeWidth={2} className="size-6 translate-x-0.5" />
            </span>
          </button>
        ) : effectiveState === "ready" ? (
          <button
            type="button"
            onClick={togglePlay}
            aria-label="Play preview"
            className="absolute inset-0 z-10 flex flex-col items-center justify-center gap-3 bg-black/60 transition-colors hover:bg-black/50"
          >
            <span className="flex size-14 items-center justify-center rounded-full bg-white/90 text-black">
              <HugeiconsIcon icon={PlayIcon} strokeWidth={2} className="size-6 translate-x-0.5" />
            </span>
            <span className="text-xs text-zinc-300">Click to play the live preview</span>
          </button>
        ) : effectiveState === "buffering" || effectiveState === "waiting-media" ? (
          <div className="absolute inset-0 z-10 flex flex-col items-center justify-center gap-2 bg-black/70">
            <HugeiconsIcon
              icon={Loading03Icon}
              strokeWidth={2}
              className="size-6 animate-spin text-zinc-300 motion-reduce:animate-none"
            />
            <p className="text-xs text-zinc-300">
              {effectiveState === "buffering"
                ? "Buffering…"
                : "Waiting for the first video segments…"}
            </p>
            {effectiveState === "waiting-media" && status === "live" && (
              <p className="text-[11px] text-zinc-500">
                OBS is connected — the playlist appears within a few seconds.
              </p>
            )}
          </div>
        ) : effectiveState === "waiting-publisher" ? (
          <div className="absolute inset-0 z-10 flex flex-col items-center justify-center gap-2 bg-black/80 px-6 text-center">
            <span className="flex size-10 items-center justify-center rounded-full bg-white/10">
              <HugeiconsIcon icon={AntennaIcon} strokeWidth={1.7} className="size-5 text-zinc-300" />
            </span>
            <p className="text-sm font-medium text-white">Waiting for OBS</p>
            <p className="max-w-xs text-xs text-zinc-400">
              Start streaming from OBS with the connection details — the preview appears here
              once the broadcast is received.
            </p>
          </div>
        ) : effectiveState === "ended" ? (
          <div className="absolute inset-0 z-10 flex flex-col items-center justify-center gap-2 bg-black/80 px-6 text-center">
            <span className="flex size-10 items-center justify-center rounded-full bg-white/10">
              <HugeiconsIcon icon={Cancel01Icon} strokeWidth={1.7} className="size-5 text-zinc-400" />
            </span>
            <p className="text-sm font-medium text-white">Stream ended</p>
            <p className="max-w-xs text-xs text-zinc-400">
              The preview is no longer available. Create a new stream to broadcast again.
            </p>
          </div>
        ) : (
          <StreamErrorState
            className="absolute inset-0 z-10 bg-black/80"
            title={
              effectiveState === "interrupted"
                ? "Stream interrupted"
                : effectiveState === "unsupported"
                  ? "Live preview is not supported in this browser"
                  : effectiveState === "invalid-url"
                    ? "No playback URL"
                    : "Playback error"
            }
            description={
              effectiveState === "interrupted"
                ? "The HLS playlist stopped responding. It may recover on its own — retry, or check the broadcast in OBS."
                : effectiveState === "unsupported"
                  ? "This browser cannot play HLS video. Use the playback URL with an external player instead."
                  : effectiveState === "invalid-url"
                    ? "This stream has no playback URL yet."
                    : "The preview could not be decoded. Retry, or open the HLS playback URL externally."
            }
            actionLabel={
              effectiveState === "interrupted" || effectiveState === "error" ? "Retry" : undefined
            }
            onAction={
              effectiveState === "interrupted" || effectiveState === "error"
                ? handleRetry
                : undefined
            }
            linkHref={hasSource ? playbackUrl : undefined}
          />
        )}
      </div>

      <div className="flex items-center gap-1 bg-zinc-950 px-2 py-1.5">
        <Button
          variant="ghost"
          size="icon-sm"
          onClick={togglePlay}
          disabled={!controlsEnabled}
          aria-label={isPlaying ? "Pause preview" : "Play preview"}
          className="text-zinc-300 hover:bg-white/10 hover:text-white disabled:text-zinc-600"
        >
          <HugeiconsIcon icon={isPlaying ? PauseIcon : PlayIcon} strokeWidth={2} className="size-4" />
        </Button>
        <Button
          variant="ghost"
          size="icon-sm"
          onClick={toggleMute}
          disabled={!controlsEnabled}
          aria-label={muted ? "Unmute preview" : "Mute preview"}
          className="text-zinc-300 hover:bg-white/10 hover:text-white disabled:text-zinc-600"
        >
          <HugeiconsIcon
            icon={muted ? VolumeMuteIcon : VolumeUpIcon}
            strokeWidth={2}
            className="size-4"
          />
        </Button>
        <input
          type="range"
          min={0}
          max={1}
          step={0.05}
          value={muted ? 0 : volume}
          onChange={(event) => handleVolumeInput(Number(event.target.value))}
          disabled={!controlsEnabled}
          aria-label="Preview volume"
          className="hidden h-1 w-20 cursor-pointer appearance-none rounded-full bg-zinc-700 accent-white sm:block disabled:cursor-not-allowed disabled:opacity-40"
        />
        <span className="ml-2 hidden text-[11px] text-zinc-500 sm:inline">
          Preview plays muted — unmute with the speaker button.
        </span>
        <div className="ml-auto flex items-center gap-0.5">
          {status === "live" && (
            <span className="mr-1.5 inline-flex items-center gap-1.5 text-[11px] font-medium text-red-400">
              <span className="size-1.5 rounded-full bg-red-500 animate-pulse motion-reduce:animate-none" />
              LIVE
            </span>
          )}
          <Button
            variant="ghost"
            size="icon-sm"
            onClick={() => window.open(playbackUrl, "_blank", "noreferrer")}
            disabled={!hasSource}
            aria-label="Open HLS playback URL in a new tab"
            className="text-zinc-300 hover:bg-white/10 hover:text-white disabled:text-zinc-600"
          >
            <HugeiconsIcon icon={ArrowUpRightIcon} strokeWidth={2} className="size-4" />
          </Button>
          <Button
            variant="ghost"
            size="icon-sm"
            onClick={toggleFullscreen}
            aria-label={isFullscreen ? "Exit fullscreen" : "Fullscreen preview"}
            className="text-zinc-300 hover:bg-white/10 hover:text-white"
          >
            <HugeiconsIcon
              icon={isFullscreen ? Minimize02Icon : Maximize01Icon}
              strokeWidth={2}
              className="size-4"
            />
          </Button>
        </div>
      </div>
    </div>
  )
}
