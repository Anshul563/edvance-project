"use client"

/* eslint-disable @next/next/no-img-element -- channel artwork is inlined as a data URL */

import { useCallback, useEffect, useRef, useState } from "react"
import Link from "next/link"
import { toast } from "sonner"
import { HugeiconsIcon } from "@hugeicons/react"
import {
  Alert02Icon,
  Camera01Icon,
  CheckmarkCircle01Icon,
  Delete02Icon,
  Image01Icon,
} from "@hugeicons/core-free-icons"

import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Textarea } from "@/components/ui/textarea"
import { Skeleton } from "@/components/ui/skeleton"
import {
  getMyChannel,
  updateMyChannel,
  type CreatorProfile,
  type UpdateChannelInput,
} from "@/components/channel/channel-api"
import { initialsOf } from "@/hooks/use-session-user"
import { imageFileToDataUrl } from "@/lib/image"

const MAX_DESCRIPTION = 1000

interface ChannelForm {
  channelName: string
  description: string
  avatarUrl: string
  bannerUrl: string
}

function formFromProfile(profile: CreatorProfile): ChannelForm {
  return {
    channelName: profile.channel.name,
    description: profile.channel.description ?? "",
    avatarUrl: profile.channel.avatarUrl ?? "",
    bannerUrl: profile.channel.bannerUrl ?? "",
  }
}

export default function ManageChannelPage() {
  const [profile, setProfile] = useState<CreatorProfile | null>(null)
  const [form, setForm] = useState<ChannelForm | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [errorStatus, setErrorStatus] = useState<number | null>(null)
  const [saving, setSaving] = useState(false)
  const [uploading, setUploading] = useState<"avatar" | "banner" | null>(null)

  const avatarInput = useRef<HTMLInputElement>(null)
  const bannerInput = useRef<HTMLInputElement>(null)

  const load = useCallback(async () => {
    setLoading(true)
    setError(null)
    const result = await getMyChannel()
    if (!result.success) {
      setError(result.error?.message ?? "Could not load your channel.")
      setErrorStatus(result.error?.status ?? null)
      setLoading(false)
      return
    }
    setErrorStatus(null)
    setProfile(result.data)
    setForm(formFromProfile(result.data))
    setLoading(false)
  }, [])

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load()
  }, [load])

  function update<K extends keyof ChannelForm>(key: K, value: ChannelForm[K]) {
    setForm((current) => (current ? { ...current, [key]: value } : current))
  }

  async function handleImage(
    kind: "avatar" | "banner",
    file: File | undefined,
    input: HTMLInputElement | null
  ) {
    if (!file || !form) return
    setUploading(kind)
    try {
      const dataUrl = await imageFileToDataUrl(
        file,
        kind === "avatar"
          ? { width: 512, aspect: 1 }
          : { width: 1280, aspect: 16 / 6 }
      )
      update(kind === "avatar" ? "avatarUrl" : "bannerUrl", dataUrl)
    } catch (err) {
      toast.error(
        err instanceof Error ? err.message : "The image could not be processed."
      )
    } finally {
      setUploading(null)
      if (input) input.value = ""
    }
  }

  async function handleSave(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!form || !profile || saving) return

    const name = form.channelName.trim()
    if (!name) {
      toast.error("Channel name is required.")
      return
    }

    if (name.length > 100) {
      toast.error("Channel name must be 100 characters or fewer.")
      return
    }

    const payload: UpdateChannelInput = {}
    if (name !== profile.channel.name) payload.channelName = name
    if (form.description !== (profile.channel.description ?? "")) {
      payload.description = form.description
    }
    if (form.avatarUrl !== (profile.channel.avatarUrl ?? "")) {
      payload.avatarUrl = form.avatarUrl
    }
    if (form.bannerUrl !== (profile.channel.bannerUrl ?? "")) {
      payload.bannerUrl = form.bannerUrl
    }

    if (Object.keys(payload).length === 0) {
      toast.info("No changes to save.")
      return
    }

    setSaving(true)
    const result = await updateMyChannel(payload)
    setSaving(false)

    if (!result.success) {
      toast.error(result.error?.message ?? "Your channel could not be updated.")
      return
    }

    setProfile(result.data)
    setForm(formFromProfile(result.data))
    toast.success("Channel updated.", {
      description: "Your public channel now shows these details.",
    })
  }

  const dirty = Boolean(
    form &&
    profile &&
    (form.channelName.trim() !== profile.channel.name ||
      form.description !== (profile.channel.description ?? "") ||
      form.avatarUrl !== (profile.channel.avatarUrl ?? "") ||
      form.bannerUrl !== (profile.channel.bannerUrl ?? ""))
  )

  return (
    <div className="mx-auto w-full max-w-5xl p-4 sm:p-6">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h2 className="text-lg font-semibold">Manage channel</h2>
          <p className="mt-1 text-xs text-muted-foreground sm:text-sm">
            Update how your channel appears to learners across Edvance.
          </p>
        </div>
        {profile ? (
          <Badge
            variant={profile.status === "active" ? "secondary" : "outline"}
          >
            {profile.status === "active" ? "Active" : profile.status}
          </Badge>
        ) : null}
      </div>

      {loading ? (
        <div className="mt-6 space-y-4">
          <Skeleton className="h-44 w-full" />
          <Skeleton className="h-64 w-full" />
        </div>
      ) : error || !form || !profile ? (
        <div className="mt-6 rounded-xl border border-destructive/30 bg-destructive/5 p-5">
          <div className="flex items-start gap-3">
            <HugeiconsIcon
              icon={Alert02Icon}
              strokeWidth={1.8}
              className="mt-0.5 size-5 shrink-0 text-destructive"
            />
            <div className="min-w-0">
              <h3 className="text-sm font-semibold">
                {errorStatus === 404
                  ? "Creator profile required"
                  : "Channel unavailable"}
              </h3>
              <p className="mt-1 text-xs text-muted-foreground">
                {errorStatus === 404
                  ? "Create your creator profile before managing channel details."
                  : (error ?? "The channel could not be loaded.")}
              </p>
              <div className="mt-4 flex flex-wrap gap-2">
                {errorStatus === 404 ? (
                  <Button size="sm" render={<Link href="/creator/videos" />}>
                    Set up creator profile
                  </Button>
                ) : (
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() => void load()}
                  >
                    Try again
                  </Button>
                )}
              </div>
            </div>
          </div>
        </div>
      ) : (
        <div className="mt-6 grid gap-6 lg:grid-cols-[minmax(0,1fr)_340px]">
          <form className="space-y-6" onSubmit={handleSave}>
            <section className="rounded-xl border border-border">
              <div className="border-b border-border px-4 py-3">
                <h3 className="text-sm font-semibold">Channel details</h3>
                <p className="mt-0.5 text-xs text-muted-foreground">
                  Name, artwork, and description shown on your public channel.
                </p>
              </div>

              <div className="space-y-6 p-4">
                <div className="flex flex-wrap items-center gap-4">
                  <div className="relative flex size-20 shrink-0 items-center justify-center overflow-hidden rounded-full bg-primary text-xl font-bold text-primary-foreground">
                    {form.avatarUrl ? (
                      <img
                        src={form.avatarUrl}
                        alt="Channel avatar preview"
                        className="size-full object-cover"
                      />
                    ) : (
                      initialsOf(form.channelName || profile.displayName)
                    )}
                  </div>
                  <div className="min-w-0 flex-1">
                    <p className="text-xs font-medium">Profile photo</p>
                    <p className="mt-1 text-xs text-muted-foreground">
                      Square JPEG, PNG, or WebP up to 8 MB.
                    </p>
                    <div className="mt-3 flex flex-wrap gap-2">
                      <Button
                        type="button"
                        size="sm"
                        variant="outline"
                        disabled={uploading !== null}
                        onClick={() => avatarInput.current?.click()}
                      >
                        <HugeiconsIcon
                          icon={Camera01Icon}
                          strokeWidth={1.8}
                          className="mr-1.5 size-3.5"
                        />
                        {uploading === "avatar"
                          ? "Processing..."
                          : "Change photo"}
                      </Button>
                      {form.avatarUrl ? (
                        <Button
                          type="button"
                          size="sm"
                          variant="ghost"
                          disabled={uploading !== null}
                          onClick={() => update("avatarUrl", "")}
                        >
                          Remove
                        </Button>
                      ) : null}
                    </div>
                    <input
                      ref={avatarInput}
                      type="file"
                      accept="image/jpeg,image/png,image/webp"
                      className="sr-only"
                      aria-label="Upload channel profile photo"
                      onChange={(event) => {
                        void handleImage(
                          "avatar",
                          event.target.files?.[0],
                          event.currentTarget
                        )
                      }}
                    />
                  </div>
                </div>

                <div className="space-y-2">
                  <div className="flex flex-wrap items-center justify-between gap-2">
                    <div>
                      <p className="text-xs font-medium">Channel banner</p>
                      <p className="mt-1 text-xs text-muted-foreground">
                        Wide image shown behind your channel header.
                      </p>
                    </div>
                    <div className="flex gap-2">
                      <Button
                        type="button"
                        size="sm"
                        variant="outline"
                        disabled={uploading !== null}
                        onClick={() => bannerInput.current?.click()}
                      >
                        <HugeiconsIcon
                          icon={Image01Icon}
                          strokeWidth={1.8}
                          className="mr-1.5 size-3.5"
                        />
                        {uploading === "banner"
                          ? "Processing..."
                          : "Change banner"}
                      </Button>
                      {form.bannerUrl ? (
                        <Button
                          type="button"
                          size="sm"
                          variant="ghost"
                          disabled={uploading !== null}
                          onClick={() => update("bannerUrl", "")}
                        >
                          <HugeiconsIcon
                            icon={Delete02Icon}
                            strokeWidth={1.8}
                            className="mr-1.5 size-3.5"
                          />
                          Remove
                        </Button>
                      ) : null}
                    </div>
                  </div>
                  {form.bannerUrl ? (
                    <img
                      src={form.bannerUrl}
                      alt="Channel banner preview"
                      className="h-28 w-full rounded-lg border border-border object-cover"
                    />
                  ) : (
                    <div className="flex h-28 w-full items-center justify-center rounded-lg border border-dashed border-border bg-muted/40">
                      <p className="text-xs text-muted-foreground">
                        No banner uploaded
                      </p>
                    </div>
                  )}
                  <input
                    ref={bannerInput}
                    type="file"
                    accept="image/jpeg,image/png,image/webp"
                    className="sr-only"
                    aria-label="Upload channel banner"
                    onChange={(event) => {
                      void handleImage(
                        "banner",
                        event.target.files?.[0],
                        event.currentTarget
                      )
                    }}
                  />
                </div>

                <div className="grid gap-4 sm:grid-cols-2">
                  <div className="grid gap-1.5">
                    <Label
                      htmlFor="channel-name"
                      className="text-xs font-medium"
                    >
                      Channel name
                    </Label>
                    <Input
                      id="channel-name"
                      value={form.channelName}
                      maxLength={100}
                      required
                      onChange={(event) =>
                        update("channelName", event.target.value)
                      }
                    />
                  </div>
                  <div className="grid gap-1.5">
                    <Label
                      htmlFor="channel-handle"
                      className="text-xs font-medium"
                    >
                      Handle
                    </Label>
                    <Input
                      id="channel-handle"
                      value={`@${profile.channel.handle}`}
                      readOnly
                      disabled
                    />
                    <p className="text-[11px] text-muted-foreground">
                      Handle changes are not available yet.
                    </p>
                  </div>
                </div>

                <div className="grid gap-1.5">
                  <div className="flex items-center justify-between gap-2">
                    <Label
                      htmlFor="channel-description"
                      className="text-xs font-medium"
                    >
                      Description
                    </Label>
                    <span className="text-[11px] text-muted-foreground">
                      {form.description.length}/{MAX_DESCRIPTION}
                    </span>
                  </div>
                  <Textarea
                    id="channel-description"
                    value={form.description}
                    maxLength={MAX_DESCRIPTION}
                    rows={5}
                    placeholder="Tell learners what your channel is about."
                    onChange={(event) =>
                      update("description", event.target.value)
                    }
                  />
                </div>
              </div>
            </section>

            <div className="flex flex-wrap items-center justify-end gap-2">
              <Button
                type="button"
                variant="ghost"
                size="sm"
                disabled={!dirty || saving}
                onClick={() => setForm(formFromProfile(profile))}
              >
                Discard changes
              </Button>
              <Button
                type="submit"
                size="sm"
                disabled={!dirty || saving || uploading !== null}
              >
                <HugeiconsIcon
                  icon={CheckmarkCircle01Icon}
                  strokeWidth={1.8}
                  className="mr-1.5 size-3.5"
                />
                {saving ? "Saving..." : "Save changes"}
              </Button>
            </div>
          </form>

          <aside className="space-y-3 self-start lg:sticky lg:top-20">
            <p className="text-xs font-medium text-muted-foreground">
              Public preview
            </p>
            <div className="overflow-hidden rounded-xl border border-border">
              <div className="h-28 w-full bg-gradient-to-br from-primary/90 via-primary/70 to-primary/40">
                {form.bannerUrl ? (
                  <img
                    src={form.bannerUrl}
                    alt=""
                    className="size-full object-cover"
                  />
                ) : null}
              </div>
              <div className="px-4 pb-4">
                <div className="-mt-8 flex size-16 items-center justify-center overflow-hidden rounded-full border-4 border-background bg-primary text-base font-bold text-primary-foreground">
                  {form.avatarUrl ? (
                    <img
                      src={form.avatarUrl}
                      alt=""
                      className="size-full object-cover"
                    />
                  ) : (
                    initialsOf(form.channelName || profile.displayName)
                  )}
                </div>
                <h3 className="mt-3 truncate text-sm font-semibold">
                  {form.channelName.trim() || "Untitled channel"}
                </h3>
                <p className="mt-0.5 text-xs text-muted-foreground">
                  @{profile.channel.handle}
                </p>
                <p className="mt-3 line-clamp-3 text-xs whitespace-pre-line text-muted-foreground">
                  {form.description.trim() || "No channel description yet."}
                </p>
              </div>
            </div>
            <p className="text-[11px] text-muted-foreground">
              This preview shows the details saved to your public channel.
            </p>
          </aside>
        </div>
      )}
    </div>
  )
}
