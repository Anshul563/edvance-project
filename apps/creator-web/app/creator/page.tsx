"use client"

import Link from "next/link"
import { useState } from "react"
import { toast } from "sonner"
import { cn } from "cn"
import { HugeiconsIcon } from "@hugeicons/react"
import {
  TrendingUp,
  Users,
  StarIcon,
  LayoutIcon,
  CheckmarkCircle01Icon,
  Add01Icon,
  DollarCircleIcon,
  ArrowRight01Icon,
  Book01Icon,
  Image01Icon,
} from "@hugeicons/core-free-icons"
import { AreaChart, Area, BarChart, Bar, XAxis, YAxis, CartesianGrid } from "recharts"
import {
  ChartContainer,
  ChartTooltip,
  ChartTooltipContent,
  type ChartConfig,
} from "@/components/ui/chart"
import { KPICard } from "@/components/kpi-card"
import { demoStore } from "@/src/lib/demo-store"
import type { RevenueOverTime } from "@/src/types/creator"

type Period = "7d" | "30d" | "90d" | "12m"

const periodLabels: Record<Period, string> = {
  "7d": "7D",
  "30d": "30D",
  "90d": "90D",
  "12m": "12M",
}

const revenueConfig: ChartConfig = {
  revenue: { label: "Revenue ($)", color: "hsl(262, 83%, 58%)" },
}
const enrollmentConfig: ChartConfig = {
  enrollments: { label: "Enrollments", color: "hsl(210, 80%, 55%)" },
}

export default function OverviewPage() {
  const [period, setPeriod] = useState<Period>("30d")

  const allCourses = demoStore.getCourses()
  const students = demoStore.getStudents()
  const reviews = demoStore.getReviews()
  const recentStudents = students.slice(0, 3)
  const topCourses = [...allCourses]
    .filter((c) => c.status === "published")
    .sort((a, b) => b.revenue - a.revenue)
    .slice(0, 3)
  const recentReviews = reviews.slice(0, 3)
  const chartData: RevenueOverTime[] = demoStore.getRevenueOverTime(period)

  const totalRevenue = allCourses.reduce((s, c) => s + c.revenue, 0)
  const totalStudents = allCourses.reduce((s, c) => s + c.students, 0)
  const publishedCount = allCourses.filter((c) => c.status === "published").length
  const avgRating = allCourses.reduce((s, c) => s + c.rating, 0) / allCourses.length
  const draftCourse = allCourses.find((c) => c.status === "draft")

  return (
    <div className="p-4 sm:p-6 space-y-6">
      {/* Welcome */}
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4 sm:gap-5">
        <div>
          <h2 className="text-xl font-semibold sm:text-2xl">Hello, Alex Morgan 👋</h2>
          <p className="mt-1 text-xs sm:text-sm text-muted-foreground">
            Here&apos;s your creator overview — <span className="font-medium">May 15, 2024</span>
          </p>
        </div>
        <div className="flex gap-3">
          <Link
            href="/creator/create"
            className="inline-flex items-center gap-1.5 border border-primary bg-primary px-4 py-2 text-xs sm:text-sm font-medium text-primary-foreground hover:bg-primary/90 transition-colors rounded-none"
          >
            <HugeiconsIcon icon={Add01Icon} strokeWidth={2} className="h-3.5 w-3.5 sm:h-4 sm:w-4" />
            Create Course
          </Link>
          <Link
            href="/creator/analytics"
            className="inline-flex items-center gap-1.5 border border-border px-4 py-2 text-xs sm:text-sm font-medium hover:bg-muted transition-colors rounded-none"
          >
            <HugeiconsIcon icon={TrendingUp} strokeWidth={2} className="h-3.5 w-3.5 sm:h-4 sm:w-4" />
            Analytics
          </Link>
        </div>
      </div>

      {/* KPI Cards */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4 sm:gap-5">
        <KPICard
          title="Total Revenue"
          value={`$${totalRevenue.toLocaleString()}`}
          comparison="+18% vs last year"
          icon={DollarCircleIcon}
        />
        <KPICard
          title="This Month"
          value="$7,230"
          comparison="+12% vs last month"
          icon={TrendingUp}
        />
        <KPICard
          title="Total Students"
          value={totalStudents.toLocaleString()}
          comparison="+247 new this month"
          icon={Users}
        />
        <KPICard
          title="Published"
          value={publishedCount}
          comparison={`${allCourses.length} total courses`}
          icon={LayoutIcon}
        />
        <KPICard
          title="Completion"
          value="73%"
          comparison="+5% vs last period"
          icon={CheckmarkCircle01Icon}
        />
        <KPICard
          title="Avg Rating"
          value={avgRating.toFixed(1)}
          comparison="out of 5.0"
          icon={StarIcon}
        />
      </div>

      {/* Charts */}
      <div className="space-y-3">
        <div className="flex items-center justify-between">
          <h3 className="text-sm font-medium">Revenue &amp; Enrollments</h3>
          <div className="flex gap-1">
            {(["7d", "30d", "90d", "12m"] as Period[]).map((p) => (
              <button
                key={p}
                onClick={() => setPeriod(p)}
                className={cn(
                  "px-2 py-0.5 text-xs border transition-colors",
                  period === p
                    ? "border-primary bg-primary text-primary-foreground"
                    : "border-border hover:bg-muted"
                )}
              >
                {periodLabels[p]}
              </button>
            ))}
          </div>
        </div>
        <div className="grid grid-cols-1 gap-5 lg:grid-cols-2">
          <div className="border border-border p-5">
            <p className="mb-3 text-xs text-muted-foreground">Revenue over time</p>
            <ChartContainer config={revenueConfig} className="h-[240px] w-full">
              <AreaChart data={chartData} margin={{ top: 4, right: 8, bottom: 0, left: 0 }}>
                <CartesianGrid strokeDasharray="3 3" vertical={false} />
                <XAxis dataKey="date" tick={{ fontSize: 10 }} tickLine={false} axisLine={false} interval="preserveStartEnd" />
                <YAxis tick={{ fontSize: 10 }} tickLine={false} axisLine={false} width={40} />
                <ChartTooltip content={<ChartTooltipContent />} />
                <Area type="monotone" dataKey="revenue" stroke="var(--color-revenue)" fill="var(--color-revenue)" fillOpacity={0.12} strokeWidth={2} />
              </AreaChart>
            </ChartContainer>
          </div>
          <div className="border border-border p-5">
            <p className="mb-3 text-xs text-muted-foreground">Student enrollments</p>
            <ChartContainer config={enrollmentConfig} className="h-[240px] w-full">
              <BarChart data={chartData} margin={{ top: 4, right: 8, bottom: 0, left: 0 }}>
                <CartesianGrid strokeDasharray="3 3" vertical={false} />
                <XAxis dataKey="date" tick={{ fontSize: 10 }} tickLine={false} axisLine={false} interval="preserveStartEnd" />
                <YAxis tick={{ fontSize: 10 }} tickLine={false} axisLine={false} width={32} />
                <ChartTooltip content={<ChartTooltipContent />} />
                <Bar dataKey="enrollments" fill="var(--color-enrollments)" radius={0} />
              </BarChart>
            </ChartContainer>
          </div>
        </div>
      </div>

      {/* Bottom grid */}
      <div className="grid grid-cols-1 gap-5 lg:grid-cols-3">
        {/* Recent enrollments */}
        <div className="border border-border p-4">
          <div className="mb-3 flex items-center justify-between">
            <h3 className="text-xs font-medium">Recent Enrollments</h3>
            <Link href="/creator/students" className="flex items-center gap-0.5 text-xs text-primary hover:underline">
              View all <HugeiconsIcon icon={ArrowRight01Icon} strokeWidth={2} className="h-3 w-3" />
            </Link>
          </div>
          <div className="space-y-2.5">
            {recentStudents.map((s) => (
              <div key={s.id} className="flex items-center gap-2.5">
                <div className="flex h-7 w-7 flex-shrink-0 items-center justify-center bg-primary/10 text-xs font-semibold text-primary">
                  {s.name.split(" ").map((n) => n[0]).join("")}
                </div>
                <div className="min-w-0 flex-1">
                  <p className="truncate text-xs font-medium">{s.name}</p>
                  <p className="truncate text-xs text-muted-foreground">{s.enrolledCourse}</p>
                </div>
              </div>
            ))}
          </div>
        </div>

        {/* Top courses */}
        <div className="border border-border p-5">
          <div className="mb-3 flex items-center justify-between">
            <h3 className="text-sm font-medium">Top Courses</h3>
            <Link href="/creator/courses" className="flex items-center gap-0.5 text-xs text-primary hover:underline">
              View all <HugeiconsIcon icon={ArrowRight01Icon} strokeWidth={2} className="h-3 w-3" />
            </Link>
          </div>
          <div className="space-y-3">
            {topCourses.map((c) => (
              <div key={c.id} className="flex items-center gap-3">
                <div className="flex h-8 w-8 flex-shrink-0 items-center justify-center bg-muted text-xs font-semibold text-muted-foreground">
                  {c.title[0]}
                </div>
                <div className="min-w-0 flex-1">
                  <p className="truncate text-sm font-medium">{c.title}</p>
                  <p className="text-xs text-muted-foreground">{c.students.toLocaleString()} students · ${c.revenue.toLocaleString()}</p>
                </div>
              </div>
            ))}
          </div>
        </div>

        {/* Recent reviews */}
        <div className="border border-border p-5">
          <div className="mb-3 flex items-center justify-between">
            <h3 className="text-sm font-medium">Recent Reviews</h3>
            <Link href="/creator/reviews" className="flex items-center gap-0.5 text-xs text-primary hover:underline">
              View all <HugeiconsIcon icon={ArrowRight01Icon} strokeWidth={2} className="h-3 w-3" />
            </Link>
          </div>
          <div className="space-y-3">
            {recentReviews.map((r) => (
              <div key={r.id} className="space-y-1">
                <div className="flex items-center gap-1">
                  {Array.from({ length: 5 }).map((_, i) => (
                    <HugeiconsIcon
                      key={i}
                      icon={StarIcon}
                      strokeWidth={2}
                      className={cn("h-3 w-3", i < r.rating ? "text-amber-400" : "text-muted-foreground/30")}
                    />
                  ))}
                  <span className="ml-1 text-xs text-muted-foreground">{r.createdAt}</span>
                </div>
                <p className="line-clamp-2 text-xs text-muted-foreground">{r.content}</p>
              </div>
            ))}
          </div>
        </div>
      </div>

      {/* Checklist + Quick actions */}
      <div className="grid grid-cols-1 gap-5 lg:grid-cols-2">
        {draftCourse && (
          <div className="border border-border p-5">
            <h3 className="mb-3 text-sm font-medium">
              Publishing checklist — <span className="text-muted-foreground">{draftCourse.title}</span>
            </h3>
            <div className="space-y-2.5">
              {[
                { label: "Course title and subtitle", done: !!draftCourse.title },
                { label: "Course description", done: !!draftCourse.description },
                { label: "Thumbnail", done: false },
                { label: "At least one lesson", done: draftCourse.lessons.length > 0 },
                { label: "Pricing configured", done: !!draftCourse.pricing },
              ].map((item) => (
                <div key={item.label} className="flex items-center gap-2.5">
                  <HugeiconsIcon
                    icon={CheckmarkCircle01Icon}
                    strokeWidth={2}
                    className={cn("h-3.5 w-3.5 flex-shrink-0", item.done ? "text-green-500" : "text-muted-foreground/40")}
                  />
                  <span className={cn("text-xs", !item.done && "text-muted-foreground")}>{item.label}</span>
                </div>
              ))}
            </div>
            <Link
              href={`/creator/courses/${draftCourse.id}/edit`}
              className="mt-3 inline-flex items-center gap-1 text-xs text-primary hover:underline"
            >
              Continue editing <HugeiconsIcon icon={ArrowRight01Icon} strokeWidth={2} className="h-3 w-3" />
            </Link>
          </div>
        )}

        <div className="border border-border p-5">
          <h3 className="mb-3 text-sm font-medium">Quick Actions</h3>
          <div className="grid grid-cols-2 gap-3">
            {[
              { label: "Create Course", icon: Add01Icon, href: "/creator/create" },
              { label: "Media Library", icon: Image01Icon, href: "/creator/media" },
              { label: "Analytics", icon: TrendingUp, href: "/creator/analytics" },
              { label: "My Courses", icon: Book01Icon, href: "/creator/courses" },
            ].map((item) => (
              <Link
                key={item.label}
                href={item.href}
                className="flex items-center gap-2.5 border border-border p-3.5 hover:bg-muted transition-colors"
              >
                <HugeiconsIcon icon={item.icon} strokeWidth={2} className="h-4 w-4 flex-shrink-0 text-primary" />
                <span className="text-xs font-medium">{item.label}</span>
              </Link>
            ))}
          </div>
        </div>
      </div>
    </div>
  )
}
