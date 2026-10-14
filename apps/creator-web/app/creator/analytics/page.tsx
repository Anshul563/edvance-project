"use client"

import { useState } from "react"
import { cn } from "cn"
import { HugeiconsIcon } from "@hugeicons/react"
import { TrendingUp, Users, StarIcon, DollarCircleIcon } from "@hugeicons/core-free-icons"
import { AreaChart, Area, BarChart, Bar, XAxis, YAxis, CartesianGrid } from "recharts"
import {
  ChartContainer, ChartTooltip, ChartTooltipContent, type ChartConfig,
} from "@/components/ui/chart"
import { KPICard } from "@/components/kpi-card"
import { demoStore } from "@/src/lib/demo-store"

type Period = "7d" | "30d" | "90d" | "12m"
const PERIODS: { value: Period; label: string }[] = [
  { value: "7d", label: "7 Days" },
  { value: "30d", label: "30 Days" },
  { value: "90d", label: "90 Days" },
  { value: "12m", label: "12 Months" },
]

const revenueConfig: ChartConfig = {
  revenue: { label: "Revenue ($)", color: "hsl(262, 83%, 58%)" },
}
const enrollmentConfig: ChartConfig = {
  enrollments: { label: "Enrollments", color: "hsl(210, 80%, 55%)" },
}

const TRAFFIC = [
  { source: "Organic Search", pct: 45, color: "bg-violet-500" },
  { source: "Direct", pct: 25, color: "bg-blue-500" },
  { source: "Referral", pct: 20, color: "bg-green-500" },
  { source: "Social Media", pct: 10, color: "bg-amber-500" },
]

const FUNNEL = [
  { label: "Visitors", count: 12480 },
  { label: "Course Page Views", count: 5420 },
  { label: "Enrolled", count: 684 },
  { label: "Completed", count: 499 },
]

type SortDir = "asc" | "desc"

export default function AnalyticsPage() {
  const [period, setPeriod] = useState<Period>("30d")
  const [sortCol, setSortCol] = useState<string>("revenue")
  const [sortDir, setSortDir] = useState<SortDir>("desc")

  const chartData = demoStore.getRevenueOverTime(period)
  const coursePerf = demoStore.getCoursePerformance()

  const periodRevenue = chartData.reduce((s, d) => s + d.revenue, 0)
  const periodEnrollments = chartData.reduce((s, d) => s + d.enrollments, 0)

  const sortedCourses = [...coursePerf].sort((a, b) => {
    const v = a[sortCol as keyof typeof a] as number
    const w = b[sortCol as keyof typeof b] as number
    return sortDir === "asc" ? (v as number) - (w as number) : (w as number) - (v as number)
  })

  function toggleSort(col: string) {
    if (sortCol === col) setSortDir((d) => d === "asc" ? "desc" : "asc")
    else { setSortCol(col); setSortDir("desc") }
  }

  return (
    <div className="p-4 sm:p-6 max-w-7xl mx-auto space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between flex-wrap gap-2">
        <div>
          <h2 className="text-lg font-semibold">Analytics</h2>
          <p className="text-xs text-muted-foreground mt-0.5">Demo data — fictional metrics</p>
        </div>
        <div className="flex gap-1">
          {PERIODS.map((p) => (
            <button
              key={p.value}
              onClick={() => setPeriod(p.value)}
              className={cn("px-2.5 py-1 text-xs border transition-colors", period === p.value ? "border-primary bg-primary text-primary-foreground" : "border-border hover:bg-muted")}
            >
              {p.label}
            </button>
          ))}
        </div>
      </div>

      {/* KPI cards */}
      <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
        <KPICard title="Period Revenue" value={`$${periodRevenue.toLocaleString()}`} icon={DollarCircleIcon} />
        <KPICard title="Enrollments" value={periodEnrollments} icon={Users} />
        <KPICard title="Avg Completion" value="73%" icon={TrendingUp} />
        <KPICard title="Avg Rating" value="4.6" icon={StarIcon} />
      </div>

      {/* Charts */}
      <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
        <div className="border border-border p-4">
          <p className="mb-3 text-xs font-medium">Revenue over time</p>
          <ChartContainer config={revenueConfig} className="h-[200px] w-full">
            <AreaChart data={chartData} margin={{ top: 4, right: 4, bottom: 0, left: 0 }}>
              <CartesianGrid strokeDasharray="3 3" vertical={false} />
              <XAxis dataKey="date" tick={{ fontSize: 9 }} tickLine={false} axisLine={false} interval="preserveStartEnd" />
              <YAxis tick={{ fontSize: 9 }} tickLine={false} axisLine={false} width={40} tickFormatter={(v) => `$${v}`} />
              <ChartTooltip content={<ChartTooltipContent />} />
              <Area type="monotone" dataKey="revenue" stroke="var(--color-revenue)" fill="var(--color-revenue)" fillOpacity={0.12} strokeWidth={2} />
            </AreaChart>
          </ChartContainer>
        </div>
        <div className="border border-border p-4">
          <p className="mb-3 text-xs font-medium">Student enrollments</p>
          <ChartContainer config={enrollmentConfig} className="h-[200px] w-full">
            <BarChart data={chartData} margin={{ top: 4, right: 4, bottom: 0, left: 0 }}>
              <CartesianGrid strokeDasharray="3 3" vertical={false} />
              <XAxis dataKey="date" tick={{ fontSize: 9 }} tickLine={false} axisLine={false} interval="preserveStartEnd" />
              <YAxis tick={{ fontSize: 9 }} tickLine={false} axisLine={false} width={28} />
              <ChartTooltip content={<ChartTooltipContent />} />
              <Bar dataKey="enrollments" fill="var(--color-enrollments)" radius={0} />
            </BarChart>
          </ChartContainer>
        </div>
      </div>

      {/* Course performance table */}
      <div className="border border-border">
        <div className="border-b border-border px-4 py-2.5">
          <p className="text-xs font-medium">Course Performance</p>
        </div>
        <div className="overflow-x-auto">
          <table className="w-full text-xs">
            <thead>
              <tr className="border-b border-border bg-muted/50">
                {[["title", "Course"], ["students", "Students"], ["revenue", "Revenue"], ["rating", "Rating"]].map(([col, label]) => (
                  <th
                    key={col}
                    className="cursor-pointer px-4 py-2 text-left font-medium text-muted-foreground hover:text-foreground"
                    onClick={() => toggleSort(col)}
                  >
                    {label} {sortCol === col ? (sortDir === "asc" ? "↑" : "↓") : ""}
                  </th>
                ))}
                <th className="px-4 py-2 text-left font-medium text-muted-foreground">Completion</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border">
              {sortedCourses.map((c) => (
                <tr key={c.id} className="hover:bg-muted/30">
                  <td className="px-4 py-2.5 font-medium">{c.title}</td>
                  <td className="px-4 py-2.5 text-muted-foreground">{c.students.toLocaleString()}</td>
                  <td className="px-4 py-2.5 text-muted-foreground">${c.revenue.toLocaleString()}</td>
                  <td className="px-4 py-2.5 text-muted-foreground">{c.rating.toFixed(1)}</td>
                  <td className="px-4 py-2.5 text-muted-foreground">{Math.floor(65 + (c.id.charCodeAt(1) % 20))}%</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      {/* Traffic + Funnel */}
      <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
        <div className="border border-border p-4">
          <p className="mb-3 text-xs font-medium">Traffic Sources</p>
          <div className="space-y-3">
            {TRAFFIC.map((t) => (
              <div key={t.source}>
                <div className="mb-1 flex justify-between text-xs">
                  <span>{t.source}</span>
                  <span className="text-muted-foreground">{t.pct}%</span>
                </div>
                <div className="h-1.5 w-full bg-muted overflow-hidden">
                  <div className={cn("h-full", t.color)} style={{ width: `${t.pct}%` }} />
                </div>
              </div>
            ))}
          </div>
        </div>
        <div className="border border-border p-4">
          <p className="mb-3 text-xs font-medium">Conversion Funnel</p>
          <div className="space-y-2">
            {FUNNEL.map((f, i) => (
              <div key={f.label} className="flex items-center gap-3">
                <span className="w-32 text-xs">{f.label}</span>
                <div className="flex-1 h-6 bg-muted overflow-hidden">
                  <div
                    className="h-full bg-primary/70 flex items-center px-2"
                    style={{ width: `${(f.count / FUNNEL[0].count) * 100}%` }}
                  >
                    <span className="text-xs text-primary-foreground font-medium">{f.count.toLocaleString()}</span>
                  </div>
                </div>
                {i > 0 && (
                  <span className="w-12 text-right text-xs text-muted-foreground">
                    {((f.count / FUNNEL[i - 1].count) * 100).toFixed(0)}%
                  </span>
                )}
              </div>
            ))}
          </div>
        </div>
      </div>
    </div>
  )
}
