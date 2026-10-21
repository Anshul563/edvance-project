"use client"

import * as React from "react"
import { HugeiconsIcon } from "@hugeicons/react"
import { Mail01Icon } from "@hugeicons/core-free-icons"
import { AppShell } from "@/components/layout/app-shell"
import { PageHeader } from "@/components/shared/page-header"
import { InitialsAvatar } from "@/components/shared/initials-avatar"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import { Card } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Textarea } from "@/components/ui/textarea"
import { useDemoState } from "@/hooks/use-demo-store"
import { saveProfile } from "@/lib/demo-store"
import { formatMonthYear } from "@/lib/format"
import { completedViews, inProgressViews } from "@/lib/progress"

const BIO_MAX_LENGTH = 280

interface ProfileDraft {
  name?: string
  headline?: string
  bio?: string
  interests?: string
  goal?: string
  weeklyGoalMinutes?: string
}

export default function ProfilePage() {
  const state = useDemoState()
  const [draft, setDraft] = React.useState<ProfileDraft>({})
  const [status, setStatus] = React.useState("")

  const profile = state.profile

  const name = draft.name ?? profile.name
  const headline = draft.headline ?? profile.headline
  const bio = draft.bio ?? profile.bio
  const interests = draft.interests ?? profile.interests.join(", ")
  const goal = draft.goal ?? profile.goal
  const weeklyGoalMinutes =
    draft.weeklyGoalMinutes ?? String(profile.weeklyGoalMinutes)

  function update(field: keyof ProfileDraft, value: string): void {
    setDraft((previous) => ({ ...previous, [field]: value }))
  }

  function handleSave(event: React.FormEvent<HTMLFormElement>): void {
    event.preventDefault()
    const parsedGoal = Number.parseInt(weeklyGoalMinutes, 10)
    saveProfile({
      name: name.trim() || profile.name,
      headline: headline.trim(),
      bio: bio.trim(),
      goal: goal.trim(),
      interests: interests
        .split(",")
        .map((item) => item.trim())
        .filter(Boolean),
      weeklyGoalMinutes: Number.isFinite(parsedGoal)
        ? Math.max(30, parsedGoal)
        : profile.weeklyGoalMinutes,
    })
    setDraft({})
    setStatus("Profile saved (demo data stays in this device).")
  }

  const stats = [
    { label: "Courses in progress", value: inProgressViews(state).length },
    { label: "Courses completed", value: completedViews(state).length },
    { label: "Certificates", value: state.certificates.length },
  ]

  return (
    <AppShell>
      <div className="mx-auto max-w-3xl space-y-6">
        <PageHeader
          title="Profile"
          description="How you appear across Edvance — saved to this browser demo."
        />

        <Card className="gap-0 border border-border py-0 ring-0">
          <div className="flex flex-col gap-4 border-b border-border px-4 py-4 sm:flex-row sm:items-center sm:justify-between">
            <div className="flex min-w-0 items-center gap-4">
              <InitialsAvatar
                name={profile.name}
                size="lg"
                className="rounded-full"
              />
              <div className="min-w-0">
                <p className="truncate text-sm font-semibold">{profile.name}</p>
                <p className="mt-0.5 truncate text-xs text-muted-foreground">
                  {profile.headline}
                </p>
                <p className="mt-1 text-[11px] text-muted-foreground">
                  Member since {formatMonthYear(profile.joinedAt)}
                </p>
              </div>
            </div>
            <dl className="flex shrink-0 gap-5">
              {stats.map((stat) => (
                <div key={stat.label}>
                  <dt className="text-[11px] text-muted-foreground">
                    {stat.label}
                  </dt>
                  <dd className="text-lg font-semibold">{stat.value}</dd>
                </div>
              ))}
            </dl>
          </div>

          <form onSubmit={handleSave} className="space-y-4 px-4 py-4">
            <div className="grid gap-4 sm:grid-cols-2">
              <div className="space-y-1.5">
                <Label htmlFor="profile-name">Full name</Label>
                <Input
                  id="profile-name"
                  value={name}
                  onChange={(event) => update("name", event.target.value)}
                />
              </div>
              <div className="space-y-1.5">
                <Label htmlFor="profile-headline">Headline</Label>
                <Input
                  id="profile-headline"
                  value={headline}
                  onChange={(event) => update("headline", event.target.value)}
                />
              </div>
            </div>

            <div className="space-y-1.5">
              <Label htmlFor="profile-email">Email</Label>
              <Input id="profile-email" value={profile.email} readOnly />
              <Alert className="mt-2">
                <HugeiconsIcon
                  icon={Mail01Icon}
                  strokeWidth={2}
                  className="size-4"
                />
                <AlertTitle>Email</AlertTitle>
                <AlertDescription>
                  Email changes require a verified account — disabled in this
                  demo.
                </AlertDescription>
              </Alert>
            </div>

            <div className="space-y-1.5">
              <div className="flex items-center justify-between gap-3">
                <Label htmlFor="profile-bio">Bio</Label>
                <span className="text-[11px] text-muted-foreground">
                  {bio.length}/{BIO_MAX_LENGTH}
                </span>
              </div>
              <Textarea
                id="profile-bio"
                rows={4}
                maxLength={BIO_MAX_LENGTH}
                value={bio}
                onChange={(event) => update("bio", event.target.value)}
              />
            </div>

            <div className="space-y-1.5">
              <Label htmlFor="profile-interests">Interests</Label>
              <Input
                id="profile-interests"
                placeholder="Web Development, UI/UX Design, Data Science"
                value={interests}
                onChange={(event) => update("interests", event.target.value)}
              />
              <p className="text-[11px] text-muted-foreground">
                Separate interests with commas — saved as a list.
              </p>
            </div>

            <div className="grid gap-4 sm:grid-cols-2">
              <div className="space-y-1.5">
                <Label htmlFor="profile-goal">Learning goal</Label>
                <Textarea
                  id="profile-goal"
                  rows={2}
                  value={goal}
                  onChange={(event) => update("goal", event.target.value)}
                />
              </div>
              <div className="space-y-1.5">
                <Label htmlFor="profile-weekly-goal">
                  Weekly goal (minutes)
                </Label>
                <Input
                  id="profile-weekly-goal"
                  type="number"
                  min={30}
                  step={30}
                  value={weeklyGoalMinutes}
                  onChange={(event) =>
                    update("weeklyGoalMinutes", event.target.value)
                  }
                />
                <p className="text-[11px] text-muted-foreground">
                  Minimum 30 minutes, steps of 30.
                </p>
              </div>
            </div>

            <div className="flex flex-wrap items-center gap-3 border-t border-border pt-4">
              <Button type="submit" size="sm" className="text-sm md:text-sm">
                Save profile
              </Button>
              <p
                role="status"
                className="text-xs text-primary dark:text-[#C4B5FD]"
              >
                {status}
              </p>
            </div>
          </form>
        </Card>
      </div>
    </AppShell>
  )
}
