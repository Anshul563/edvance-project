"use client"

import { useState } from "react"
import { useRouter } from "next/navigation"
import { toast } from "sonner"
import { cn } from "cn"
import { HugeiconsIcon } from "@hugeicons/react"
import {
  CheckmarkCircle01Icon,
  Add01Icon,
  Delete01Icon,
  ArrowLeft01Icon,
  ArrowRight01Icon,
  FileUploadIcon,
} from "@hugeicons/core-free-icons"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Textarea } from "@/components/ui/textarea"
import { Label } from "@/components/ui/label"
import type { Section, Lesson } from "@/src/types/creator"

const STEPS = [
  { n: 1, label: "Basic Info" },
  { n: 2, label: "Media" },
  { n: 3, label: "Curriculum" },
  { n: 4, label: "Pricing" },
  { n: 5, label: "Publish" },
]

const CATEGORIES = ["Programming", "Design", "Business", "Marketing", "Data Science", "Lifestyle", "Arts", "Music"]
const DIFFICULTIES = ["beginner", "intermediate", "advanced"]
const LANGUAGES = [{ value: "en", label: "English" }, { value: "es", label: "Spanish" }, { value: "fr", label: "French" }, { value: "de", label: "German" }]

type FormData = {
  title: string; subtitle: string; description: string; category: string
  difficulty: string; language: string; learningObjectives: string[]; prerequisites: string[]
  intendedAudience: string; thumbnail: string; promotionalVideo: string
  sections: Section[]; pricing: "free" | "paid"; price: string; discountPrice: string
}

const initialForm: FormData = {
  title: "", subtitle: "", description: "", category: "Programming",
  difficulty: "beginner", language: "en", learningObjectives: [], prerequisites: [],
  intendedAudience: "", thumbnail: "", promotionalVideo: "",
  sections: [], pricing: "free", price: "", discountPrice: "",
}

function TagInput({ label, tags, onChange }: { label: string; tags: string[]; onChange: (tags: string[]) => void }) {
  const [input, setInput] = useState("")
  function add() {
    const v = input.trim()
    if (v && !tags.includes(v)) { onChange([...tags, v]); setInput("") }
  }
  return (
    <div>
      <Label className="text-xs font-medium">{label}</Label>
      <div className="mt-1 flex gap-1.5">
        <Input value={input} onChange={(e) => setInput(e.target.value)} onKeyDown={(e) => { if (e.key === "Enter") { e.preventDefault(); add() } }} placeholder="Type and press Enter" className="flex-1 text-xs" />
        <Button type="button" variant="outline" size="sm" onClick={add}>Add</Button>
      </div>
      {tags.length > 0 && (
        <div className="mt-2 flex flex-wrap gap-1.5">
          {tags.map((t) => (
            <span key={t} className="flex items-center gap-1 border border-border bg-muted px-1.5 py-0.5 text-xs">
              {t}
              <button onClick={() => onChange(tags.filter((x) => x !== t))} className="text-muted-foreground hover:text-foreground">×</button>
            </span>
          ))}
        </div>
      )}
    </div>
  )
}

export default function CreateCoursePage() {
  const router = useRouter()
  const [step, setStep] = useState(1)
  const [form, setForm] = useState<FormData>(initialForm)

  function set<K extends keyof FormData>(key: K, value: FormData[K]) {
    setForm((f) => ({ ...f, [key]: value }))
  }

  function addSection() {
    const s: Section = { id: `s-${Date.now()}`, title: "New Section", lessons: [] }
    set("sections", [...form.sections, s])
  }

  function updateSection(id: string, title: string) {
    set("sections", form.sections.map((s) => s.id === id ? { ...s, title } : s))
  }

  function removeSection(id: string) {
    set("sections", form.sections.filter((s) => s.id !== id))
  }

  function addLesson(sectionId: string) {
    const l: Lesson = { id: `l-${Date.now()}`, title: "New Lesson", type: "video", preview: false, order: 0 }
    set("sections", form.sections.map((s) =>
      s.id === sectionId ? { ...s, lessons: [...s.lessons, l] } : s
    ))
  }

  function updateLesson(sectionId: string, lessonId: string, updates: Partial<Lesson>) {
    set("sections", form.sections.map((s) =>
      s.id === sectionId
        ? { ...s, lessons: s.lessons.map((l) => l.id === lessonId ? { ...l, ...updates } : l) }
        : s
    ))
  }

  function removeLesson(sectionId: string, lessonId: string) {
    set("sections", form.sections.map((s) =>
      s.id === sectionId ? { ...s, lessons: s.lessons.filter((l) => l.id !== lessonId) } : s
    ))
  }

  const checklistItems = [
    { label: "Course title", done: form.title.trim().length > 0 },
    { label: "Description", done: form.description.trim().length > 0 },
    { label: "Thumbnail", done: false },
    { label: "At least one section", done: form.sections.length > 0 },
    { label: "Pricing configured", done: form.pricing === "free" || form.price.length > 0 },
  ]

  function save(asStatus: "draft" | "review" | "publish") {
    const labels = { draft: "Saved as draft", review: "Submitted for review", publish: "Course published" }
    toast.success(labels[asStatus], { description: "Demo — no data was persisted to a server." })
    setTimeout(() => router.push("/creator/courses"), 800)
  }

  return (
    <div className="p-4 sm:p-6 max-w-3xl mx-auto">
      {/* Header */}
      <div className="mb-6">
        <h2 className="text-lg font-semibold">Create New Course</h2>
        <p className="text-xs text-muted-foreground">Fill in the details below to create and publish your course.</p>
      </div>

      {/* Step indicator */}
      <div className="mb-8 flex items-center gap-0">
        {STEPS.map((s, i) => (
          <div key={s.n} className="flex items-center">
            <button
              onClick={() => step > s.n && setStep(s.n)}
              className={cn(
                "flex h-7 w-7 items-center justify-center text-xs font-semibold transition-colors",
                step === s.n ? "bg-primary text-primary-foreground" : step > s.n ? "bg-primary/20 text-primary" : "bg-muted text-muted-foreground"
              )}
            >
              {step > s.n ? <HugeiconsIcon icon={CheckmarkCircle01Icon} strokeWidth={2} className="h-4 w-4" /> : s.n}
            </button>
            <span className={cn("ml-1.5 mr-3 hidden text-xs sm:block", step === s.n ? "font-medium" : "text-muted-foreground")}>{s.label}</span>
            {i < STEPS.length - 1 && <div className={cn("h-px w-6 sm:w-12 flex-shrink-0", step > s.n ? "bg-primary/40" : "bg-border")} />}
          </div>
        ))}
      </div>

      {/* Step content */}
      <div className="min-h-[400px]">
        {step === 1 && (
          <div className="space-y-4">
            <div>
              <Label htmlFor="title" className="text-xs font-medium">Course Title <span className="text-destructive">*</span></Label>
              <Input id="title" value={form.title} onChange={(e) => set("title", e.target.value)} placeholder="e.g. Complete Python Bootcamp" className="mt-1" />
            </div>
            <div>
              <Label htmlFor="subtitle" className="text-xs font-medium">Subtitle</Label>
              <Input id="subtitle" value={form.subtitle} onChange={(e) => set("subtitle", e.target.value)} placeholder="Brief, catchy subtitle" className="mt-1" />
            </div>
            <div>
              <Label htmlFor="desc" className="text-xs font-medium">Description</Label>
              <Textarea id="desc" value={form.description} onChange={(e) => set("description", e.target.value)} placeholder="Detailed course description" className="mt-1" rows={4} />
            </div>
            <div className="grid grid-cols-2 gap-4 sm:grid-cols-3">
              {[
                { id: "category", label: "Category", opts: CATEGORIES },
                { id: "difficulty", label: "Difficulty", opts: DIFFICULTIES },
                { id: "language", label: "Language", opts: LANGUAGES.map((l) => l.value) },
              ].map(({ id, label, opts }) => (
                <div key={id}>
                  <Label htmlFor={id} className="text-xs font-medium">{label}</Label>
                  <select
                    id={id}
                    value={form[id as keyof FormData] as string}
                    onChange={(e) => set(id as keyof FormData, e.target.value as never)}
                    className="mt-1 h-8 w-full rounded-none border border-border bg-background px-2 text-xs outline-none focus:ring-1 focus:ring-ring/50"
                  >
                    {opts.map((o) => <option key={o} value={o}>{o}</option>)}
                  </select>
                </div>
              ))}
            </div>
            <TagInput label="Learning Objectives" tags={form.learningObjectives} onChange={(t) => set("learningObjectives", t)} />
            <TagInput label="Prerequisites" tags={form.prerequisites} onChange={(t) => set("prerequisites", t)} />
            <div>
              <Label htmlFor="audience" className="text-xs font-medium">Intended Audience</Label>
              <Input id="audience" value={form.intendedAudience} onChange={(e) => set("intendedAudience", e.target.value)} placeholder="Who is this course for?" className="mt-1" />
            </div>
          </div>
        )}

        {step === 2 && (
          <div className="space-y-6">
            {[{ key: "thumbnail", label: "Course Thumbnail", hint: "Recommended: 1280×720px, JPEG or PNG" }, { key: "promotionalVideo", label: "Promotional Video", hint: "Max 2 minutes. Shown on course landing page." }].map(({ key, label, hint }) => (
              <div key={key}>
                <Label className="text-xs font-medium">{label}</Label>
                <p className="text-xs text-muted-foreground">{hint}</p>
                <div
                  className="mt-2 flex h-36 cursor-pointer flex-col items-center justify-center gap-2 border border-dashed border-border hover:bg-muted/50 transition-colors"
                  onClick={() => toast.info("Demo", { description: "File upload is simulated. No files are uploaded in this demo." })}
                >
                  <HugeiconsIcon icon={FileUploadIcon} strokeWidth={1.5} className="h-8 w-8 text-muted-foreground" />
                  <p className="text-xs text-muted-foreground">Click to upload (demo)</p>
                </div>
                {form.title && (
                  <div className="mt-2 flex h-20 items-center justify-center bg-primary/10 text-2xl font-bold text-primary">
                    {form.title[0]?.toUpperCase()}
                  </div>
                )}
              </div>
            ))}
          </div>
        )}

        {step === 3 && (
          <div className="space-y-4">
            <div className="flex items-center justify-between">
              <p className="text-xs text-muted-foreground">{form.sections.length} section{form.sections.length !== 1 ? "s" : ""}</p>
              <Button size="sm" variant="outline" onClick={addSection}>
                <HugeiconsIcon icon={Add01Icon} strokeWidth={2} className="mr-1 h-3.5 w-3.5" />
                Add Section
              </Button>
            </div>
            {form.sections.length === 0 && (
              <div className="flex h-40 items-center justify-center border border-dashed border-border">
                <p className="text-xs text-muted-foreground">No sections yet. Click &quot;Add Section&quot; to start.</p>
              </div>
            )}
            {form.sections.map((section) => (
              <div key={section.id} className="border border-border">
                <div className="flex items-center gap-2 bg-muted/50 px-3 py-2">
                  <Input
                    value={section.title}
                    onChange={(e) => updateSection(section.id, e.target.value)}
                    className="flex-1 bg-transparent border-0 p-0 text-xs font-medium focus:ring-0 h-auto"
                  />
                  <Button size="icon-xs" variant="ghost" onClick={() => removeSection(section.id)} aria-label="Remove section">
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
                      <label className="flex items-center gap-1 text-xs text-muted-foreground">
                        <input
                          type="checkbox"
                          checked={lesson.preview}
                          onChange={(e) => updateLesson(section.id, lesson.id, { preview: e.target.checked })}
                        />
                        Preview
                      </label>
                      <Button size="icon-xs" variant="ghost" onClick={() => removeLesson(section.id, lesson.id)} aria-label="Remove lesson">
                        <HugeiconsIcon icon={Delete01Icon} strokeWidth={2} className="h-3 w-3 text-destructive" />
                      </Button>
                    </div>
                  ))}
                </div>
                <div className="px-3 py-2">
                  <Button size="xs" variant="ghost" onClick={() => addLesson(section.id)}>
                    <HugeiconsIcon icon={Add01Icon} strokeWidth={2} className="mr-1 h-3 w-3" />
                    Add Lesson
                  </Button>
                </div>
              </div>
            ))}
          </div>
        )}

        {step === 4 && (
          <div className="space-y-6 max-w-sm">
            <div>
              <p className="mb-3 text-xs font-medium">Course Pricing</p>
              <div className="flex gap-2">
                {(["free", "paid"] as const).map((p) => (
                  <button
                    key={p}
                    onClick={() => set("pricing", p)}
                    className={cn("flex-1 border py-2 text-xs font-medium transition-colors", form.pricing === p ? "border-primary bg-primary text-primary-foreground" : "border-border hover:bg-muted")}
                  >
                    {p === "free" ? "Free" : "Paid"}
                  </button>
                ))}
              </div>
            </div>
            {form.pricing === "paid" && (
              <>
                <div>
                  <Label htmlFor="price" className="text-xs font-medium">Price (USD)</Label>
                  <div className="mt-1 flex items-center gap-1">
                    <span className="text-xs text-muted-foreground">$</span>
                    <Input id="price" type="number" value={form.price} onChange={(e) => set("price", e.target.value)} placeholder="49.99" className="w-32" />
                  </div>
                </div>
                <div>
                  <Label htmlFor="discountPrice" className="text-xs font-medium">Discount Price (optional)</Label>
                  <div className="mt-1 flex items-center gap-1">
                    <span className="text-xs text-muted-foreground">$</span>
                    <Input id="discountPrice" type="number" value={form.discountPrice} onChange={(e) => set("discountPrice", e.target.value)} placeholder="29.99" className="w-32" />
                  </div>
                </div>
              </>
            )}
            <div className="border border-border p-3">
              <p className="text-xs font-medium mb-1">Pricing Summary</p>
              <p className="text-xs text-muted-foreground">
                {form.pricing === "free" ? "Free course" : form.price ? `$${form.price} per enrollment${form.discountPrice ? ` (was $${form.discountPrice})` : ""}` : "Set a price above"}
              </p>
            </div>
          </div>
        )}

        {step === 5 && (
          <div className="space-y-6">
            <div>
              <h3 className="text-sm font-medium mb-3">Publishing Checklist</h3>
              <div className="space-y-2">
                {checklistItems.map((item) => (
                  <div key={item.label} className="flex items-center gap-2">
                    <HugeiconsIcon
                      icon={CheckmarkCircle01Icon}
                      strokeWidth={2}
                      className={cn("h-4 w-4 flex-shrink-0", item.done ? "text-green-500" : "text-muted-foreground/40")}
                    />
                    <span className={cn("text-xs", !item.done && "text-muted-foreground")}>{item.label}</span>
                    {!item.done && <span className="ml-auto text-xs text-destructive">Required</span>}
                  </div>
                ))}
              </div>
            </div>
            <div className="border border-border p-4">
              <h3 className="text-xs font-medium mb-2">Course Summary</h3>
              <dl className="space-y-1 text-xs">
                <div className="flex gap-2"><dt className="text-muted-foreground w-24 flex-shrink-0">Title:</dt><dd>{form.title || "—"}</dd></div>
                <div className="flex gap-2"><dt className="text-muted-foreground w-24 flex-shrink-0">Category:</dt><dd>{form.category}</dd></div>
                <div className="flex gap-2"><dt className="text-muted-foreground w-24 flex-shrink-0">Difficulty:</dt><dd>{form.difficulty}</dd></div>
                <div className="flex gap-2"><dt className="text-muted-foreground w-24 flex-shrink-0">Sections:</dt><dd>{form.sections.length}</dd></div>
                <div className="flex gap-2"><dt className="text-muted-foreground w-24 flex-shrink-0">Pricing:</dt><dd>{form.pricing === "free" ? "Free" : form.price ? `$${form.price}` : "Not set"}</dd></div>
              </dl>
            </div>
            <div className="flex flex-wrap gap-2">
              <Button variant="outline" size="sm" onClick={() => save("draft")}>Save as Draft</Button>
              <Button variant="secondary" size="sm" onClick={() => save("review")}>Submit for Review</Button>
              <Button size="sm" onClick={() => save("publish")} disabled={checklistItems.some((i) => !i.done && i.label !== "Thumbnail")}>Publish Now</Button>
            </div>
            <p className="text-xs text-muted-foreground">Demo: No data is sent to a server. All changes are local to this session.</p>
          </div>
        )}
      </div>

      {/* Navigation */}
      <div className="mt-8 flex items-center justify-between border-t border-border pt-4">
        <Button variant="outline" size="sm" onClick={() => step > 1 ? setStep((s) => s - 1) : router.push("/creator/courses")} disabled={false}>
          <HugeiconsIcon icon={ArrowLeft01Icon} strokeWidth={2} className="mr-1 h-3.5 w-3.5" />
          {step === 1 ? "Cancel" : "Previous"}
        </Button>
        {step < 5 && (
          <Button size="sm" onClick={() => setStep((s) => s + 1)}>
            Next
            <HugeiconsIcon icon={ArrowRight01Icon} strokeWidth={2} className="ml-1 h-3.5 w-3.5" />
          </Button>
        )}
      </div>
    </div>
  )
}
