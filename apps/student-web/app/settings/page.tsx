"use client"

import * as React from "react"
import { useRouter } from "next/navigation"
import { HugeiconsIcon } from "@hugeicons/react"
import { Logout01Icon, Refresh01Icon } from "@hugeicons/core-free-icons"
import { AppShell } from "@/components/layout/app-shell"
import { PageHeader } from "@/components/shared/page-header"
import { InitialsAvatar } from "@/components/shared/initials-avatar"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Switch } from "@/components/ui/switch"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { useDemoState } from "@/hooks/use-demo-store"
import {
  demoSignOut,
  resetDemo,
  saveProfile,
  setPreferences,
  type DemoPreferences,
} from "@/lib/demo-store"
import { cn } from "@/lib/utils"

const NOTIFICATION_ROWS: {
  key: keyof DemoPreferences
  label: string
  description: string
}[] = [
  {
    key: "emailNotifications",
    label: "Course updates by email",
    description:
      "Email when a course you enrolled in gets new or revised lessons.",
  },
  {
    key: "weeklyDigest",
    label: "Weekly progress digest",
    description: "A Sunday recap of minutes studied and lessons finished.",
  },
  {
    key: "remindersEnabled",
    label: "Study reminders",
    description: "Nudges when your streak or weekly goal is about to slip.",
  },
]

export default function SettingsPage() {
  const state = useDemoState()
  const router = useRouter()

  const session = state.session
  const account = session ?? state.profile

  const [displayName, setDisplayName] = React.useState<string | null>(null)
  const [weeklyGoal, setWeeklyGoal] = React.useState<string | null>(null)
  const [accountStatus, setAccountStatus] = React.useState("")
  const [notificationStatus, setNotificationStatus] = React.useState("")
  const [learningStatus, setLearningStatus] = React.useState("")
  const [dataStatus, setDataStatus] = React.useState("")
  const [resetOpen, setResetOpen] = React.useState(false)

  const nameValue = displayName ?? account.name
  const weeklyGoalValue = weeklyGoal ?? String(state.profile.weeklyGoalMinutes)

  function handleNameSave(event: React.FormEvent<HTMLFormElement>): void {
    event.preventDefault()
    saveProfile({ name: nameValue.trim() || state.profile.name })
    setDisplayName(null)
    setAccountStatus("Display name saved.")
  }

  function handleSignOut(): void {
    demoSignOut()
    setDisplayName(null)
    setAccountStatus("")
    router.push("/")
  }

  function handlePreferenceChange(
    key: keyof DemoPreferences,
    checked: boolean
  ): void {
    const patch = { [key]: checked } as Partial<DemoPreferences>
    setPreferences(patch)
    setNotificationStatus("Preference saved.")
  }

  function handleWeeklyGoalSave(event: React.FormEvent<HTMLFormElement>): void {
    event.preventDefault()
    const parsed = Number.parseInt(weeklyGoalValue, 10)
    saveProfile({
      weeklyGoalMinutes: Number.isFinite(parsed)
        ? Math.max(30, parsed)
        : state.profile.weeklyGoalMinutes,
    })
    setWeeklyGoal(null)
    setLearningStatus("Weekly goal saved.")
  }

  function handleResetConfirm(): void {
    resetDemo()
    setResetOpen(false)
    setDisplayName(null)
    setWeeklyGoal(null)
    setAccountStatus("")
    setNotificationStatus("")
    setLearningStatus("")
    setDataStatus("Demo data reset to the seeded state.")
  }

  return (
    <AppShell>
      <div className="mx-auto max-w-3xl space-y-6">
        <PageHeader
          title="Settings"
          description="Account, notifications, learning preferences, and the demo data stored in this browser."
        />

        <Tabs defaultValue="account">
          <TabsList className="h-10 w-full justify-start gap-1 overflow-x-auto border border-border bg-transparent p-1 sm:w-auto">
            <TabsTrigger value="account" className="text-sm md:text-sm">
              Account
            </TabsTrigger>
            <TabsTrigger value="notifications" className="text-sm md:text-sm">
              Notifications
            </TabsTrigger>
            <TabsTrigger value="learning" className="text-sm md:text-sm">
              Learning
            </TabsTrigger>
            <TabsTrigger value="data" className="text-sm md:text-sm">
              Data
            </TabsTrigger>
          </TabsList>

          <TabsContent value="account" className="mt-4 space-y-4">
            <div className="border border-border bg-card">
              <div className="flex items-center gap-3 px-4 py-4">
                <InitialsAvatar name={account.name} />
                <div className="min-w-0">
                  <p className="truncate text-sm font-medium">{account.name}</p>
                  <p className="truncate text-xs text-muted-foreground">
                    {account.email}
                  </p>
                </div>
              </div>
              {!session && (
                <p className="border-t border-border px-4 py-3 text-xs leading-relaxed text-muted-foreground">
                  You are browsing demo learner data. Sign in to create a local
                  demo session.
                </p>
              )}
            </div>

            <div className="border border-border bg-card">
              <div className="px-4 pt-4">
                <p className="text-sm font-medium">Change display name</p>
                <p className="mt-1 text-xs text-muted-foreground">
                  Shown across your learner pages and certificates.
                </p>
              </div>
              <form
                onSubmit={handleNameSave}
                className="flex flex-wrap items-end gap-2 px-4 py-4"
              >
                <div className="min-w-52 flex-1 space-y-1.5">
                  <Label htmlFor="settings-display-name">Display name</Label>
                  <Input
                    id="settings-display-name"
                    value={nameValue}
                    onChange={(event) => setDisplayName(event.target.value)}
                  />
                </div>
                <Button type="submit" size="sm" className="text-sm md:text-sm">
                  Save
                </Button>
                <p
                  role="status"
                  className="w-full text-xs text-primary dark:text-[#C4B5FD]"
                >
                  {accountStatus}
                </p>
              </form>
              {session && (
                <div className="flex flex-wrap items-center gap-3 border-t border-border px-4 py-3.5">
                  <Button
                    variant="outline"
                    size="sm"
                    className="text-sm md:text-sm"
                    onClick={handleSignOut}
                  >
                    <HugeiconsIcon icon={Logout01Icon} strokeWidth={2} />
                    Sign out
                  </Button>
                  <p className="text-xs text-muted-foreground">
                    Ends the local demo session on this device.
                  </p>
                </div>
              )}
            </div>
          </TabsContent>

          <TabsContent value="notifications" className="mt-4 space-y-3">
            <div className="border border-border bg-card">
              {NOTIFICATION_ROWS.map((row, index) => (
                <div
                  key={row.key}
                  className={cn(
                    "flex items-center justify-between gap-4 px-4 py-3.5",
                    index > 0 && "border-t border-border"
                  )}
                >
                  <div className="min-w-0">
                    <Label
                      htmlFor={`pref-${row.key}`}
                      className="text-sm font-medium"
                    >
                      {row.label}
                    </Label>
                    <p className="mt-1 text-xs leading-relaxed text-muted-foreground">
                      {row.description}
                    </p>
                  </div>
                  <Switch
                    id={`pref-${row.key}`}
                    checked={state.prefs[row.key]}
                    onCheckedChange={(checked) =>
                      handlePreferenceChange(row.key, checked)
                    }
                  />
                </div>
              ))}
            </div>
            <p
              role="status"
              className="text-xs text-primary dark:text-[#C4B5FD]"
            >
              {notificationStatus}
            </p>
          </TabsContent>

          <TabsContent value="learning" className="mt-4 space-y-4">
            <div className="border border-border bg-card">
              <div className="flex items-center justify-between gap-4 px-4 py-3.5">
                <div className="min-w-0">
                  <Label
                    htmlFor="pref-autoplay-video"
                    className="text-sm font-medium"
                  >
                    Autoplay the next lesson in the player
                  </Label>
                  <p className="mt-1 text-xs leading-relaxed text-muted-foreground">
                    Continue straight into the next lesson when the current one
                    finishes.
                  </p>
                </div>
                <Switch
                  id="pref-autoplay-video"
                  checked={state.prefs.autoplayVideo}
                  onCheckedChange={(checked) => {
                    setPreferences({ autoplayVideo: checked })
                    setLearningStatus("Preference saved.")
                  }}
                />
              </div>
            </div>

            <div className="border border-border bg-card px-4 py-4">
              <form
                onSubmit={handleWeeklyGoalSave}
                className="flex flex-wrap items-end gap-3"
              >
                <div className="space-y-1.5">
                  <Label htmlFor="settings-weekly-goal">
                    Weekly goal (minutes)
                  </Label>
                  <Input
                    id="settings-weekly-goal"
                    type="number"
                    min={30}
                    step={30}
                    className="w-36"
                    value={weeklyGoalValue}
                    onChange={(event) => setWeeklyGoal(event.target.value)}
                  />
                </div>
                <Button type="submit" size="sm" className="text-sm md:text-sm">
                  Save goal
                </Button>
              </form>
              <p
                role="status"
                className="mt-2 text-xs text-primary dark:text-[#C4B5FD]"
              >
                {learningStatus}
              </p>
            </div>
          </TabsContent>

          <TabsContent value="data" className="mt-4 space-y-4">
            <div className="border border-border bg-card px-4 py-4">
              <p className="text-sm font-medium">Where demo data lives</p>
              <p className="mt-2 text-xs leading-relaxed text-muted-foreground">
                Everything you change is stored in this browser under the
                localStorage key{" "}
                <code className="font-mono text-foreground">
                  edvance-student-demo-v1
                </code>
                . It holds progress, notes, wishlist, profile edits, and
                preferences only — never passwords or other credentials.
              </p>
            </div>

            <div className="flex flex-wrap items-center gap-3">
              <Dialog open={resetOpen} onOpenChange={setResetOpen}>
                <DialogTrigger
                  render={
                    <Button
                      variant="destructive"
                      size="sm"
                      className="text-sm md:text-sm"
                    />
                  }
                >
                  <HugeiconsIcon icon={Refresh01Icon} strokeWidth={2} />
                  Reset demo data
                </DialogTrigger>
                <DialogContent>
                  <DialogHeader>
                    <DialogTitle>Reset demo data?</DialogTitle>
                    <DialogDescription>
                      Progress, notes, wishlist, preferences, and profile edits
                      stored in this browser are cleared and replaced with the
                      seeded demo state. This cannot be undone.
                    </DialogDescription>
                  </DialogHeader>
                  <DialogFooter>
                    <DialogClose
                      render={
                        <Button
                          variant="outline"
                          size="sm"
                          className="text-sm md:text-sm"
                        />
                      }
                    >
                      Cancel
                    </DialogClose>
                    <Button
                      variant="destructive"
                      size="sm"
                      className="text-sm md:text-sm"
                      onClick={handleResetConfirm}
                    >
                      Reset demo data
                    </Button>
                  </DialogFooter>
                </DialogContent>
              </Dialog>
              <p
                role="status"
                className="text-xs text-primary dark:text-[#C4B5FD]"
              >
                {dataStatus}
              </p>
            </div>
          </TabsContent>
        </Tabs>
      </div>
    </AppShell>
  )
}
