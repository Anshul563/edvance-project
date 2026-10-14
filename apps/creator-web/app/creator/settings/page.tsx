"use client"

import { useState } from "react"
import { toast } from "sonner"
import { cn } from "cn"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Textarea } from "@/components/ui/textarea"
import { Label } from "@/components/ui/label"
import { Switch } from "@/components/ui/switch"
import { Separator } from "@/components/ui/separator"

type Tab = "profile" | "account" | "notifications" | "appearance" | "payout" | "security"
const TABS: { id: Tab; label: string }[] = [
  { id: "profile", label: "Profile" },
  { id: "account", label: "Account" },
  { id: "notifications", label: "Notifications" },
  { id: "appearance", label: "Appearance" },
  { id: "payout", label: "Payout" },
  { id: "security", label: "Security" },
]

export default function SettingsPage() {
  const [tab, setTab] = useState<Tab>("profile")

  // Profile state
  const [profile, setProfile] = useState({
    name: "Alex Morgan",
    tagline: "Full-Stack Developer & Educator",
    bio: "Teaching programming and productivity skills to help developers level up their careers.",
    website: "https://alexmorgan.dev",
    twitter: "@alexmorgandev",
    linkedin: "https://linkedin.com/in/alexmorgandev",
    github: "https://github.com/alexmorgan",
  })

  // Account state
  const [account, setAccount] = useState({
    displayName: "Alex Morgan",
    language: "en",
    timezone: "America/New_York",
  })

  // Notification prefs
  const [notifPrefs, setNotifPrefs] = useState({
    newEnrollment: true,
    newReview: true,
    courseApproved: true,
    earningsPayout: true,
    marketing: false,
    platformUpdates: true,
  })

  // Appearance
  const [theme, setTheme] = useState<"light" | "dark" | "system">("light")

  // Security
  const [passwords, setPasswords] = useState({ current: "", next: "", confirm: "" })

  function applyTheme(t: typeof theme) {
    setTheme(t)
    if (t === "dark") document.documentElement.classList.add("dark")
    else document.documentElement.classList.remove("dark")
  }

  return (
    <div className="p-4 sm:p-6 max-w-4xl mx-auto">
      <h2 className="mb-6 text-lg font-semibold">Settings</h2>

      <div className="flex flex-col gap-6 md:flex-row">
        {/* Sidebar nav */}
        <nav className="flex flex-row gap-1 overflow-x-auto md:flex-col md:w-36 md:flex-shrink-0">
          {TABS.map((t) => (
            <button
              key={t.id}
              onClick={() => setTab(t.id)}
              className={cn(
                "px-3 py-1.5 text-left text-xs font-medium transition-colors whitespace-nowrap",
                tab === t.id ? "bg-primary text-primary-foreground" : "hover:bg-muted"
              )}
            >
              {t.label}
            </button>
          ))}
        </nav>

        {/* Content */}
        <div className="flex-1 min-w-0 space-y-5">
          {tab === "profile" && (
            <>
              <div className="flex items-center gap-4">
                <div className="flex h-16 w-16 items-center justify-center bg-primary text-primary-foreground text-xl font-bold">AM</div>
                <Button variant="outline" size="sm" onClick={() => toast.info("Demo", { description: "Photo upload is simulated." })}>Change Photo</Button>
              </div>
              {[
                { id: "p-name", label: "Name", value: profile.name, key: "name" },
                { id: "p-tagline", label: "Tagline", value: profile.tagline, key: "tagline" },
                { id: "p-website", label: "Website", value: profile.website, key: "website" },
                { id: "p-twitter", label: "Twitter", value: profile.twitter, key: "twitter" },
                { id: "p-linkedin", label: "LinkedIn", value: profile.linkedin, key: "linkedin" },
                { id: "p-github", label: "GitHub", value: profile.github, key: "github" },
              ].map(({ id, label, value, key }) => (
                <div key={id}>
                  <Label htmlFor={id} className="text-xs font-medium">{label}</Label>
                  <Input id={id} value={value} onChange={(e) => setProfile((p) => ({ ...p, [key]: e.target.value }))} className="mt-1" />
                </div>
              ))}
              <div>
                <Label htmlFor="p-bio" className="text-xs font-medium">Bio</Label>
                <Textarea id="p-bio" value={profile.bio} onChange={(e) => setProfile((p) => ({ ...p, bio: e.target.value }))} rows={3} className="mt-1" />
              </div>
              <Button size="sm" onClick={() => toast.success("Profile saved (demo)")}>Save Profile</Button>
            </>
          )}

          {tab === "account" && (
            <>
              <div>
                <Label className="text-xs font-medium">Email</Label>
                <p className="mt-1 text-xs text-muted-foreground bg-muted px-3 py-2">alex.morgan@edvance.io</p>
                <p className="mt-1 text-xs text-muted-foreground">Email cannot be changed in this demo.</p>
              </div>
              <div>
                <Label htmlFor="a-name" className="text-xs font-medium">Display Name</Label>
                <Input id="a-name" value={account.displayName} onChange={(e) => setAccount((a) => ({ ...a, displayName: e.target.value }))} className="mt-1" />
              </div>
              {[
                { id: "a-lang", label: "Language", value: account.language, key: "language", opts: [{ v: "en", l: "English" }, { v: "es", l: "Spanish" }, { v: "fr", l: "French" }] },
                { id: "a-tz", label: "Timezone", value: account.timezone, key: "timezone", opts: [{ v: "America/New_York", l: "Eastern Time" }, { v: "America/Los_Angeles", l: "Pacific Time" }, { v: "Europe/London", l: "London" }, { v: "Asia/Kolkata", l: "India" }] },
              ].map(({ id, label, value, key, opts }) => (
                <div key={id}>
                  <Label htmlFor={id} className="text-xs font-medium">{label}</Label>
                  <select id={id} value={value} onChange={(e) => setAccount((a) => ({ ...a, [key]: e.target.value }))} className="mt-1 h-8 w-full rounded-none border border-border bg-background px-2 text-xs outline-none focus:ring-1 focus:ring-ring/50">
                    {opts.map((o) => <option key={o.v} value={o.v}>{o.l}</option>)}
                  </select>
                </div>
              ))}
              <Button size="sm" onClick={() => toast.success("Account saved (demo)")}>Save Account</Button>
            </>
          )}

          {tab === "notifications" && (
            <>
              <p className="text-xs text-muted-foreground">Manage which notifications you receive. (Demo — preferences are not persisted.)</p>
              <div className="space-y-4">
                {[
                  { key: "newEnrollment", label: "New enrollment", desc: "When a student enrolls in your course" },
                  { key: "newReview", label: "New review", desc: "When a student leaves a review" },
                  { key: "courseApproved", label: "Course approved", desc: "When your course passes review" },
                  { key: "earningsPayout", label: "Earnings payout", desc: "When a payout is processed" },
                  { key: "marketing", label: "Marketing emails", desc: "Tips, platform news, and promotions" },
                  { key: "platformUpdates", label: "Platform updates", desc: "Feature announcements and maintenance" },
                ].map(({ key, label, desc }) => (
                  <div key={key} className="flex items-center justify-between gap-4">
                    <div>
                      <p className="text-xs font-medium">{label}</p>
                      <p className="text-xs text-muted-foreground">{desc}</p>
                    </div>
                    <Switch
                      checked={notifPrefs[key as keyof typeof notifPrefs]}
                      onCheckedChange={(v) => setNotifPrefs((p) => ({ ...p, [key]: v }))}
                    />
                  </div>
                ))}
              </div>
              <Button size="sm" onClick={() => toast.success("Notification preferences saved (demo)")}>Save Preferences</Button>
            </>
          )}

          {tab === "appearance" && (
            <>
              <div>
                <p className="mb-3 text-xs font-medium">Theme</p>
                <div className="flex gap-2">
                  {(["light", "dark", "system"] as const).map((t) => (
                    <button
                      key={t}
                      onClick={() => applyTheme(t)}
                      className={cn("flex-1 border py-2 text-xs font-medium capitalize transition-colors", theme === t ? "border-primary bg-primary text-primary-foreground" : "border-border hover:bg-muted")}
                    >
                      {t}
                    </button>
                  ))}
                </div>
                <p className="mt-2 text-xs text-muted-foreground">Appearance is applied immediately but not persisted across sessions in this demo.</p>
              </div>
            </>
          )}

          {tab === "payout" && (
            <>
              <div className="border border-amber-200 bg-amber-50 px-3 py-2 dark:bg-amber-900/20 dark:border-amber-800">
                <p className="text-xs font-medium text-amber-800 dark:text-amber-400">⚠ Demo Only</p>
                <p className="mt-0.5 text-xs text-amber-700 dark:text-amber-500">No real bank details are collected or stored. This form is for demonstration purposes only.</p>
              </div>
              {[
                { id: "bank-name", label: "Bank Name", placeholder: "e.g. Chase Bank" },
                { id: "account-num", label: "Account Number", placeholder: "••••••••1234" },
                { id: "routing-num", label: "Routing Number", placeholder: "123456789" },
              ].map(({ id, label, placeholder }) => (
                <div key={id}>
                  <Label htmlFor={id} className="text-xs font-medium">{label}</Label>
                  <Input id={id} placeholder={placeholder} className="mt-1" />
                </div>
              ))}
              <Button size="sm" variant="outline" onClick={() => toast.success("Payout settings saved (demo)", { description: "No real data was stored." })}>
                Save Payout Settings (Demo)
              </Button>
            </>
          )}

          {tab === "security" && (
            <>
              <div className="border border-muted px-3 py-2">
                <p className="text-xs font-medium">Change Password</p>
                <p className="mt-0.5 text-xs text-muted-foreground">Demo — no actual authentication changes are made.</p>
              </div>
              {[
                { id: "pw-cur", label: "Current Password", key: "current" },
                { id: "pw-new", label: "New Password", key: "next" },
                { id: "pw-confirm", label: "Confirm New Password", key: "confirm" },
              ].map(({ id, label, key }) => (
                <div key={id}>
                  <Label htmlFor={id} className="text-xs font-medium">{label}</Label>
                  <Input id={id} type="password" value={passwords[key as keyof typeof passwords]} onChange={(e) => setPasswords((p) => ({ ...p, [key]: e.target.value }))} className="mt-1" />
                </div>
              ))}
              <Button size="sm" onClick={() => { setPasswords({ current: "", next: "", confirm: "" }); toast.success("Password change simulated (demo)") }}>
                Change Password (Demo)
              </Button>
              <Separator />
              <div>
                <p className="mb-3 text-xs font-medium">Active Sessions</p>
                <div className="space-y-2">
                  {[
                    { device: "Chrome on macOS", location: "San Francisco, CA", time: "Current session" },
                    { device: "Safari on iPhone 14", location: "San Francisco, CA", time: "1 hour ago" },
                  ].map((s) => (
                    <div key={s.device} className="flex items-center justify-between gap-3 border border-border p-3">
                      <div>
                        <p className="text-xs font-medium">{s.device}</p>
                        <p className="text-xs text-muted-foreground">{s.location} · {s.time}</p>
                      </div>
                      <Button size="xs" variant="outline" onClick={() => toast.success("Session revoked (demo)")}>Revoke</Button>
                    </div>
                  ))}
                </div>
              </div>
            </>
          )}
        </div>
      </div>
    </div>
  )
}
