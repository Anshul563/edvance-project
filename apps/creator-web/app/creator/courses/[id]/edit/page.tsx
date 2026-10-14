"use client"

import { useState } from "react"
import { useParams, useRouter } from "next/navigation"
import { toast } from "sonner"
import Link from "next/link"
import { cn } from "cn"
import { HugeiconsIcon } from "@hugeicons/react"
import {
  ArrowLeft01Icon,
  Add01Icon,
  Delete01Icon,
  FileUploadIcon,
  CheckmarkCircle01Icon,
} from "@hugeicons/core-free-icons"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Textarea } from "@/components/ui/textarea"
import { Label } from "@/components/ui/label"
import { StatusBadge } from "@/components/status-badge"
import { demoStore } from "@/src/lib/demo-store"
import type { Course, Lesson, Section } from "@/src/types/creator"

type Tab = "info" | "curriculum" | "media" | "pricing" | "publishing"
const TABS: { id: Tab; label: string }[] = [
  { id: "info", label: "Info" },
  { id: "curriculum", label: "Curriculum" },
  { id: "media", label: "Media" },
  { id: "pricing", label: "Pricing" },
  { id: "publishing", label: "Publishing" },
]

const CATEGORIES = ["Programming", "Design", "Business", "Marketing", "Data Science", "Lifestyle", "Arts", "Music"]
const DIFFICULTIES = ["beginner", "intermediate", "advanced"]

export default function CourseEditPage() {
  const params = useParams()
  const router = useRouter()
  const id = params.id as string
  const original = demoStore.getCourseById(id)

  const [course, setCourse] = useState<Course | null>(original ? { ...original } : null)
  const [sections, setSections] = useState<Section[]>(() => {
    const s = demoStore.getSections(id)
    if (s.length > 0) return s
    // Build sections from flat lessons
    if (original && original.lessons.length > 0) {
      return [{ id: "s-default", title: "Section 1", lessons: [...original.lessons] }]
    }
    return []
  })
  const [tab, setTab] = useState<Tab>("info")
  const [saved, setSaved] = useState(false)

  if (!course) {
    return (
      <div className="p-6 text-center">
        <p className="text-sm text-muted-foreground">Course not found.</p>
        <Link href="/creator/courses" className="mt-4 inline-flex items-center gap-1 text-xs text-primary hover:underline">
          <HugeiconsIcon icon={ArrowLeft01Icon} strokeWidth={2} className="h-3 w-3" /> Back to courses
        </Link>
      </div>
    )
  }

  function update<K extends keyof Course>(key: K, value: Course[K]) {
    setCourse((c) => c ? { ...c, [key]: value } : c)
  }

  function addSection() {
    setSections((ss) => [...ss, { id: `s-${Date.now()}`, title: "New Section", lessons: [] }])
  }

  function updateSection(sid: string, title: string) {
    setSections((ss) => ss.map((s) => s.id === sid ? { ...s, title } : s))
  }

  function removeSection(sid: string) {
    setSections((ss) => ss.filter((s) => s.id !== sid))
  }

  function addLesson(sid: string) {
    const l: Lesson = { id: `l-${Date.now()}`, title: "New Lesson", type: "video", preview: false, order: 0 }
    setSections((ss) => ss.map((s) => s.id === sid ? { ...s, lessons: [...s.lessons, l] } : s))
  }

  function updateLesson(sid: string, lid: string, changes: Partial<Lesson>) {
    setSections((ss) => ss.map((s) =>
      s.id === sid ? { ...s, lessons: s.lessons.map((l) => l.id === lid ? { ...l, ...changes } : l) } : s
    ))
  }

  function removeLesson(sid: string, lid: string) {
    setSections((ss) => ss.map((s) => s.id === sid ? { ...s, lessons: s.lessons.filter((l) => l.id !== lid) } : s))
  }

  function save() {
    demoStore.setSections(id, sections)
    setSaved(true)
    toast.success("Changes saved", { description: "Demo — not persisted to a server." })
  }

  const checklist = [
    { label: "Title and subtitle", done: course.title.length > 0 },
    { label: "Description", done: course.description.length > 0 },
    { label: "Thumbnail", done: false },
    { label: "Curriculum", done: sections.length > 0 && sections.some((s) => s.lessons.length > 0) },
    { label: "Pricing", done: !!course.pricing },
  ]

  return (
    <div className="flex min-h-full flex-col">
      {/* Header */}
      <div className="sticky top-0 z-10 border-b border-border bg-background/95 px-4 py-3 backdrop-blur-sm">
        <div className="flex items-center gap-3 flex-wrap">
          <Link href="/creator/courses" className="text-muted-foreground hover:text-foreground">
            <HugeiconsIcon icon={ArrowLeft01Icon} strokeWidth={2} className="h-4 w-4" />
          </Link>
          <div className="flex-1 min-w-0">
            <p className="text-sm font-medium truncate">{course.title}</p>
          </div>
          <StatusBadge status={course.status} />
          <Button size="sm" onClick={save}>Save Changes</Button>
        </div>
        {/* Tabs */}
        <div className="mt-2 flex gap-0 overflow-x-auto">
          {TABS.map((t) => (
            <button
              key={t.id}
              onClick={() => setTab(t.id)}
              className={cn(
                "px-3 py-1.5 text-xs font-medium border-b-2 transition-colors whitespace-nowrap",
                tab === t.id ? "border-primary text-foreground" : "border-transparent text-muted-foreground hover:text-foreground"
              )}
            >
              {t.label}
            </button>
          ))}
        </div>
      </div>

      {/* Tab content */}
      <div className="flex-1 p-4 sm:p-6 max-w-3xl mx-auto w-full">
        {tab === "info" && (
          <div className="space-y-4">
            <div>
              <Label htmlFor="edit-title" className="text-xs font-medium">Title</Label>
              <Input id="edit-title" value={course.title} onChange={(e) => update("title", e.target.value)} className="mt-1" />
            </div>
            <div>
              <Label htmlFor="edit-subtitle" className="text-xs font-medium">Subtitle</Label>
              <Input id="edit-subtitle" value={course.subtitle} onChange={(e) => update("subtitle", e.target.value)} className="mt-1" />
            </div>
            <div>
              <Label htmlFor="edit-desc" className="text-xs font-medium">Description</Label>
              <Textarea id="edit-desc" value={course.description} onChange={(e) => update("description", e.target.value)} rows={5} className="mt-1" />
            </div>
            <div className="grid grid-cols-2 gap-4 sm:grid-cols-3">
              {[
                { id: "category", label: "Category", opts: CATEGORIES, value: course.category },
                { id: "difficulty", label: "Difficulty", opts: DIFFICULTIES, value: course.difficulty },
              ].map(({ id: fid, label, opts, value }) => (
                <div key={fid}>
                  <Label htmlFor={`edit-${fid}`} className="text-xs font-medium">{label}</Label>
                  <select
                    id={`edit-${fid}`}
                    value={value}
                    onChange={(e) => update(fid as keyof Course, e.target.value as never)}
                    className="mt-1 h-8 w-full rounded-none border border-border bg-background px-2 text-xs outline-none focus:ring-1 focus:ring-ring/50"
                  >
                    {opts.map((o) => <option key={o} value={o}>{o}</option>)}
                  </select>
                </div>
              ))}
            </div>
          </div>
        )}

        {tab === "curriculum" && (
          <div className="space-y-4">
            <div className="flex items-center justify-between">
              <p className="text-xs text-muted-foreground">{sections.length} sections · {sections.reduce((n, s) => n + s.lessons.length, 0)} lessons</p>
              <Button size="sm" variant="outline" onClick={addSection}>
                <HugeiconsIcon icon={Add01Icon} strokeWidth={2} className="mr-1 h-3.5 w-3.5" /> Add Section
              </Button>
            </div>
            {sections.length === 0 && (
              <div className="flex h-40 items-center justify-center border border-dashed border-border">
                <p className="text-xs text-muted-foreground">No sections. Click &quot;Add Section&quot; to start.</p>
              </div>
            )}
            {sections.map((section) => (
              <div key={section.id} className="border border-border">
                <div className="flex items-center gap-2 bg-muted/50 px-3 py-2">
                  <Input
                    value={section.title}
                    onChange={(e) => updateSection(section.id, e.target.value)}
                    className="flex-1 h-7 bg-transparent border-0 p-0 text-xs font-medium focus:ring-0"
                  />
                  <Button size="icon-xs" variant="ghost" onClick={() => removeSection(section.id)}>
                    <HugeiconsIcon icon={Delete01Icon} strokeWidth={2} className="h-3.5 w-3.5 text-destructive" />
                  </Button>
                </div>
                <div className="divide-y divide-border/50">
                  {section.lessons.map((lesson) => (
                    <div key={lesson.id} className="flex items-center gap-2 px-3 py-2">
                      <Input
                        value={lesson.title}
                        onChange={(e) => updateLesson(section.id, lesson.id, { title: e.target.value })}
                        className="flex-1 text-xs h-7"
                      />
                      <select
                        value={lesson.type}
                        onChange={(e) => updateLesson(section.id, lesson.id, { type: e.target.value as Lesson["type"] })}
                        className="h-7 rounded-none border border-border bg-background px-1 text-xs outline-none"
                      >
                        {["video", "article", "quiz", "resource"].map((t) => <option key={t} value={t}>{t}</option>)}
                      </select>
                      <label className="flex items-center gap-1 text-xs">
                        <input type="checkbox" checked={lesson.preview} onChange={(e) => updateLesson(section.id, lesson.id, { preview: e.target.checked })} />
                        <span className="text-muted-foreground">Preview</span>
                      </label>
                      <Button size="icon-xs" variant="ghost" onClick={() => removeLesson(section.id, lesson.id)}>
                        <HugeiconsIcon icon={Delete01Icon} strokeWidth={2} className="h-3 w-3 text-destructive" />
                      </Button>
                    </div>
                  ))}
                </div>
                <div className="px-3 py-2">
                  <Button size="xs" variant="ghost" onClick={() => addLesson(section.id)}>
                    <HugeiconsIcon icon={Add01Icon} strokeWidth={2} className="mr-1 h-3 w-3" /> Add Lesson
                  </Button>
                </div>
              </div>
            ))}
          </div>
        )}

        {tab === "media" && (
          <div className="space-y-6">
            <div>
              <Label className="text-xs font-medium">Course Thumbnail</Label>
              <div className="mt-2 flex h-48 cursor-pointer flex-col items-center justify-center gap-2 border border-dashed border-border hover:bg-muted/50" onClick={() => toast.info("Demo", { description: "File upload is simulated." })}>
                <HugeiconsIcon icon={FileUploadIcon} strokeWidth={1.5} className="h-10 w-10 text-muted-foreground" />
                <p className="text-xs text-muted-foreground">Click to upload thumbnail (demo)</p>
                <p className="text-xs text-muted-foreground">Recommended: 1280×720px</p>
              </div>
            </div>
            <div>
              <Label className="text-xs font-medium">Promotional Video</Label>
              <div className="mt-2 flex h-36 cursor-pointer flex-col items-center justify-center gap-2 border border-dashed border-border hover:bg-muted/50" onClick={() => toast.info("Demo", { description: "File upload is simulated." })}>
                <HugeiconsIcon icon={FileUploadIcon} strokeWidth={1.5} className="h-8 w-8 text-muted-foreground" />
                <p className="text-xs text-muted-foreground">Click to upload video (demo)</p>
              </div>
            </div>
          </div>
        )}

        {tab === "pricing" && (
          <div className="space-y-4 max-w-sm">
            <div>
              <Label className="text-xs font-medium">Pricing Type</Label>
              <div className="mt-2 flex gap-2">
                {(["free", "paid"] as const).map((p) => (
                  <button key={p} onClick={() => update("pricing", p)} className={cn("flex-1 border py-2 text-xs font-medium", course.pricing === p ? "border-primary bg-primary text-primary-foreground" : "border-border hover:bg-muted")}>
                    {p === "free" ? "Free" : "Paid"}
                  </button>
                ))}
              </div>
            </div>
            {course.pricing === "paid" && (
              <>
                <div>
                  <Label className="text-xs font-medium">Price (USD)</Label>
                  <div className="mt-1 flex items-center gap-1">
                    <span className="text-xs text-muted-foreground">$</span>
                    <Input type="number" value={course.price ?? ""} onChange={(e) => update("price", Number(e.target.value))} className="w-32" />
                  </div>
                </div>
                <div>
                  <Label className="text-xs font-medium">Discount Price (optional)</Label>
                  <div className="mt-1 flex items-center gap-1">
                    <span className="text-xs text-muted-foreground">$</span>
                    <Input type="number" value={course.discountPrice ?? ""} onChange={(e) => update("discountPrice", Number(e.target.value))} className="w-32" />
                  </div>
                </div>
              </>
            )}
          </div>
        )}

        {tab === "publishing" && (
          <div className="space-y-6">
            <div>
              <p className="text-xs font-medium mb-1">Current Status</p>
              <StatusBadge status={course.status} />
            </div>
            <div>
              <h3 className="text-xs font-medium mb-3">Publishing Checklist</h3>
              <div className="space-y-2">
                {checklist.map((item) => (
                  <div key={item.label} className="flex items-center gap-2">
                    <HugeiconsIcon icon={CheckmarkCircle01Icon} strokeWidth={2} className={cn("h-4 w-4 flex-shrink-0", item.done ? "text-green-500" : "text-muted-foreground/40")} />
                    <span className={cn("text-xs", !item.done && "text-muted-foreground")}>{item.label}</span>
                  </div>
                ))}
              </div>
            </div>
            <div className="flex flex-wrap gap-2">
              <Button size="sm" onClick={save}>Save Changes</Button>
              {course.status === "draft" && (
                <Button variant="secondary" size="sm" onClick={() => { update("status", "in-review"); toast.success("Submitted for review (demo)") }}>
                  Submit for Review
                </Button>
              )}
              {(course.status === "draft" || course.status === "in-review") && (
                <Button variant="outline" size="sm" onClick={() => { update("status", "published"); toast.success("Course published (demo)") }}>
                  Publish
                </Button>
              )}
              {course.status === "published" && (
                <Button variant="outline" size="sm" onClick={() => { update("status", "draft"); toast.success("Course unpublished") }}>
                  Unpublish
                </Button>
              )}
            </div>
          </div>
        )}
      </div>
    </div>
  )
}
