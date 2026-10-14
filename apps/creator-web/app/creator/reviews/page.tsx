"use client"

import { useState, useMemo } from "react"
import { toast } from "sonner"
import { cn } from "cn"
import { HugeiconsIcon } from "@hugeicons/react"
import { StarIcon, ReplyIcon } from "@hugeicons/core-free-icons"
import { Button } from "@/components/ui/button"
import { Textarea } from "@/components/ui/textarea"
import { SearchFilter } from "@/components/search-filter"
import { EmptyState } from "@/components/empty-state"
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter } from "@/components/ui/dialog"
import { demoStore } from "@/src/lib/demo-store"
import type { Review } from "@/src/types/creator"

function Stars({ rating, size = "sm" }: { rating: number; size?: "sm" | "lg" }) {
  const cls = size === "lg" ? "h-5 w-5" : "h-3.5 w-3.5"
  return (
    <div className="flex gap-0.5">
      {Array.from({ length: 5 }).map((_, i) => (
        <HugeiconsIcon key={i} icon={StarIcon} strokeWidth={2} className={cn(cls, i < rating ? "text-amber-400" : "text-muted-foreground/25")} />
      ))}
    </div>
  )
}

function initials(name: string) {
  return name.split(" ").map((n) => n[0]).join("").toUpperCase()
}

export default function ReviewsPage() {
  const courses = demoStore.getCourses()
  const students = demoStore.getStudents()
  const [reviews, setReviews] = useState<Review[]>(() => demoStore.getReviews())
  const [search, setSearch] = useState("")
  const [courseFilter, setCourseFilter] = useState("all")
  const [ratingFilter, setRatingFilter] = useState<number | null>(null)
  const [replyTarget, setReplyTarget] = useState<Review | null>(null)
  const [replyText, setReplyText] = useState("")

  const courseOptions = [{ id: "all", title: "All Courses" }, ...courses]

  function studentName(id: string) {
    return students.find((s) => s.id === id)?.name ?? "Student"
  }

  function courseName(id: string) {
    return courses.find((c) => c.id === id)?.title ?? id
  }

  const filtered = useMemo(() => {
    let list = reviews
    if (search) list = list.filter((r) => r.content.toLowerCase().includes(search.toLowerCase()) || r.title.toLowerCase().includes(search.toLowerCase()))
    if (courseFilter !== "all") list = list.filter((r) => r.courseId === courseFilter)
    if (ratingFilter !== null) list = list.filter((r) => r.rating === ratingFilter)
    return list
  }, [reviews, search, courseFilter, ratingFilter])

  const avgRating = reviews.length > 0
    ? reviews.reduce((s, r) => s + r.rating, 0) / reviews.length
    : 0

  const ratingCounts = [5, 4, 3, 2, 1].map((n) => ({
    n, count: reviews.filter((r) => r.rating === n).length,
    pct: reviews.length ? Math.round((reviews.filter((r) => r.rating === n).length / reviews.length) * 100) : 0,
  }))

  function submitReply() {
    if (!replyTarget || !replyText.trim()) return
    setReviews((rs) => rs.map((r) => r.id === replyTarget.id ? { ...r, creatorReply: replyText.trim() } : r))
    toast.success("Reply posted", { description: "Demo — reply is not sent to the student." })
    setReplyTarget(null)
    setReplyText("")
  }

  return (
    <div className="p-4 sm:p-6 max-w-5xl mx-auto space-y-6">
      <div>
        <h2 className="text-lg font-semibold">Reviews</h2>
        <p className="text-xs text-muted-foreground mt-0.5">{reviews.length} total reviews (demo data)</p>
      </div>

      {/* Summary */}
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <div className="border border-border p-4 flex items-center gap-4">
          <div className="text-center">
            <p className="text-4xl font-bold">{avgRating.toFixed(1)}</p>
            <Stars rating={Math.round(avgRating)} size="lg" />
            <p className="mt-1 text-xs text-muted-foreground">{reviews.length} reviews</p>
          </div>
          <div className="flex-1 space-y-1.5">
            {ratingCounts.map(({ n, count, pct }) => (
              <div key={n} className="flex items-center gap-2">
                <button
                  onClick={() => setRatingFilter(ratingFilter === n ? null : n)}
                  className={cn("flex items-center gap-0.5 text-xs hover:opacity-80", ratingFilter === n && "font-medium")}
                >
                  {n}<HugeiconsIcon icon={StarIcon} strokeWidth={2} className="h-3 w-3 text-amber-400" />
                </button>
                <div className="flex-1 h-1.5 bg-muted overflow-hidden">
                  <div className="h-full bg-amber-400" style={{ width: `${pct}%` }} />
                </div>
                <span className="w-6 text-right text-xs text-muted-foreground">{count}</span>
              </div>
            ))}
          </div>
        </div>
        <div className="border border-border p-4">
          <p className="text-xs font-medium mb-3">Filters</p>
          <div className="space-y-2">
            <SearchFilter placeholder="Search reviews..." onChange={setSearch} />
            <select
              value={courseFilter}
              onChange={(e) => setCourseFilter(e.target.value)}
              className="h-8 w-full rounded-none border border-border bg-background px-2 text-xs outline-none focus:ring-1 focus:ring-ring/50"
            >
              {courseOptions.map((c) => <option key={c.id} value={c.id}>{c.title.length > 35 ? c.title.slice(0, 35) + "…" : c.title}</option>)}
            </select>
          </div>
        </div>
      </div>

      {/* Review list */}
      {filtered.length === 0 ? (
        <EmptyState title="No reviews match" description="Try adjusting your filters." />
      ) : (
        <div className="space-y-4">
          {filtered.map((review) => (
            <div key={review.id} className="border border-border p-4 space-y-3">
              <div className="flex items-start gap-3">
                <div className="flex h-8 w-8 flex-shrink-0 items-center justify-center bg-primary/10 text-xs font-semibold text-primary">
                  {initials(studentName(review.studentId))}
                </div>
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2 flex-wrap">
                    <span className="text-xs font-medium">{studentName(review.studentId)}</span>
                    <Stars rating={review.rating} />
                    <span className="text-xs text-muted-foreground">{review.createdAt}</span>
                  </div>
                  <p className="mt-0.5 text-xs text-muted-foreground">{courseName(review.courseId)}</p>
                  <p className="mt-2 text-xs font-medium">{review.title}</p>
                  <p className="mt-1 text-xs text-muted-foreground leading-relaxed">{review.content}</p>
                </div>
              </div>

              {review.creatorReply && (
                <div className="ml-11 border-l-2 border-primary/20 pl-3">
                  <p className="text-xs font-medium text-primary">Your reply:</p>
                  <p className="mt-0.5 text-xs text-muted-foreground">{review.creatorReply}</p>
                </div>
              )}

              <div className="ml-11">
                <Button
                  size="xs"
                  variant="ghost"
                  onClick={() => { setReplyTarget(review); setReplyText(review.creatorReply ?? "") }}
                >
                  <HugeiconsIcon icon={ReplyIcon} strokeWidth={2} className="mr-1 h-3 w-3" />
                  {review.creatorReply ? "Edit Reply" : "Reply"}
                </Button>
              </div>
            </div>
          ))}
        </div>
      )}

      {/* Reply dialog */}
      <Dialog open={!!replyTarget} onOpenChange={(o) => !o && setReplyTarget(null)}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>{replyTarget?.creatorReply ? "Edit Reply" : "Reply to Review"}</DialogTitle>
            <DialogDescription>
              Demo — replies are not sent to students.
            </DialogDescription>
          </DialogHeader>
          {replyTarget && (
            <div className="py-2 space-y-3">
              <div className="border border-border p-2 text-xs text-muted-foreground">
                <p className="font-medium text-foreground">{replyTarget.title}</p>
                <p className="mt-1 line-clamp-2">{replyTarget.content}</p>
              </div>
              <Textarea
                value={replyText}
                onChange={(e) => setReplyText(e.target.value)}
                placeholder="Write your reply..."
                rows={4}
              />
            </div>
          )}
          <DialogFooter>
            <Button variant="outline" size="sm" onClick={() => setReplyTarget(null)}>Cancel</Button>
            <Button size="sm" onClick={submitReply} disabled={!replyText.trim()}>
              Post Reply (Demo)
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
