"use client"

import { useState, useMemo } from "react"
import { toast } from "sonner"
import { cn } from "cn"
import { HugeiconsIcon } from "@hugeicons/react"
import { DollarCircleIcon, TrendingUp } from "@hugeicons/core-free-icons"
import { BarChart, Bar, XAxis, YAxis, CartesianGrid } from "recharts"
import { ChartContainer, ChartTooltip, ChartTooltipContent, type ChartConfig } from "@/components/ui/chart"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter } from "@/components/ui/dialog"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { demoStore } from "@/src/lib/demo-store"
import type { Transaction, Payout } from "@/src/types/creator"

const MONTHLY_DATA = [
  { month: "Jan", revenue: 3200 },
  { month: "Feb", revenue: 4100 },
  { month: "Mar", revenue: 3800 },
  { month: "Apr", revenue: 5200 },
  { month: "May", revenue: 7230 },
  { month: "Jun", revenue: 4900 },
]

const chartConfig: ChartConfig = { revenue: { label: "Revenue ($)", color: "hsl(262, 83%, 58%)" } }

const statusCls: Record<Transaction["status"], string> = {
  completed: "bg-green-100 text-green-700",
  pending: "bg-yellow-100 text-yellow-700",
  failed: "bg-red-100 text-red-700",
  refunded: "bg-gray-100 text-gray-600",
}

const payoutStatusCls: Record<Payout["status"], string> = {
  completed: "bg-green-100 text-green-700",
  pending: "bg-yellow-100 text-yellow-700",
  failed: "bg-red-100 text-red-700",
}

const PAGE_SIZE = 10

export default function EarningsPage() {
  const transactions = demoStore.getTransactions()
  const payouts = demoStore.getPayouts()
  const courses = demoStore.getCourses()

  const [dateFilter, setDateFilter] = useState<"all" | "month" | "3months">("all")
  const [statusFilter, setStatusFilter] = useState<string>("all")
  const [page, setPage] = useState(1)
  const [payoutDialogOpen, setPayoutDialogOpen] = useState(false)

  const summary = { available: 1245.50, pending: 289.75, lifetime: 58970 }

  function courseTitle(id: string) {
    return courses.find((c) => c.id === id)?.title ?? id
  }

  const filtered = useMemo(() => {
    let list = transactions
    if (statusFilter !== "all") list = list.filter((t) => t.status === statusFilter)
    if (dateFilter === "month") list = list.filter((t) => t.createdAt.startsWith("2024-05"))
    if (dateFilter === "3months") list = list.filter((t) => t.createdAt >= "2024-03")
    return list
  }, [transactions, statusFilter, dateFilter])

  const totalPages = Math.max(1, Math.ceil(filtered.length / PAGE_SIZE))
  const paginated = filtered.slice((page - 1) * PAGE_SIZE, page * PAGE_SIZE)

  const courseEarnings = courses.filter((c) => c.status === "published").map((c) => ({
    id: c.id, title: c.title,
    earned: transactions.filter((t) => t.courseId === c.id && t.status === "completed").reduce((s, t) => s + t.amount, 0),
    students: c.students,
    avgPerStudent: c.students > 0 ? Math.round(c.revenue / c.students) : 0,
  })).sort((a, b) => b.earned - a.earned)

  return (
    <div className="p-4 sm:p-6 max-w-7xl mx-auto space-y-6">
      <div>
        <h2 className="text-lg font-semibold">Earnings</h2>
        <p className="text-xs text-muted-foreground mt-0.5">Demo data — fictional earnings figures</p>
      </div>

      {/* Summary cards */}
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
        <div className="border border-border p-4">
          <p className="text-xs text-muted-foreground">Available Balance</p>
          <p className="mt-1 text-2xl font-semibold">${summary.available.toFixed(2)}</p>
          <Button size="xs" className="mt-3" onClick={() => setPayoutDialogOpen(true)}>Request Payout</Button>
        </div>
        <div className="border border-border p-4">
          <p className="text-xs text-muted-foreground">Pending</p>
          <p className="mt-1 text-2xl font-semibold">${summary.pending.toFixed(2)}</p>
          <p className="mt-1 text-xs text-muted-foreground">Processing 1–3 business days</p>
        </div>
        <div className="border border-border p-4">
          <p className="text-xs text-muted-foreground">Lifetime Earnings</p>
          <p className="mt-1 text-2xl font-semibold">${summary.lifetime.toLocaleString()}</p>
          <p className="mt-1 text-xs text-muted-foreground">Since September 2023</p>
        </div>
      </div>

      {/* Monthly revenue chart */}
      <div className="border border-border p-4">
        <p className="mb-3 text-xs font-medium">Monthly Revenue (Jan–Jun 2024)</p>
        <ChartContainer config={chartConfig} className="h-[180px] w-full">
          <BarChart data={MONTHLY_DATA} margin={{ top: 4, right: 4, bottom: 0, left: 0 }}>
            <CartesianGrid strokeDasharray="3 3" vertical={false} />
            <XAxis dataKey="month" tick={{ fontSize: 10 }} tickLine={false} axisLine={false} />
            <YAxis tick={{ fontSize: 10 }} tickLine={false} axisLine={false} width={40} tickFormatter={(v) => `$${v}`} />
            <ChartTooltip content={<ChartTooltipContent />} />
            <Bar dataKey="revenue" fill="var(--color-revenue)" />
          </BarChart>
        </ChartContainer>
      </div>

      {/* Transactions */}
      <div>
        <div className="mb-3 flex flex-wrap items-center gap-2">
          <p className="text-xs font-medium">Transactions</p>
          <div className="ml-auto flex gap-1 flex-wrap">
            {[{ v: "all", l: "All Time" }, { v: "month", l: "This Month" }, { v: "3months", l: "3 Months" }].map(({ v, l }) => (
              <button key={v} onClick={() => { setDateFilter(v as typeof dateFilter); setPage(1) }} className={cn("px-2 py-0.5 text-xs border", dateFilter === v ? "border-primary bg-primary text-primary-foreground" : "border-border hover:bg-muted")}>
                {l}
              </button>
            ))}
            <select value={statusFilter} onChange={(e) => { setStatusFilter(e.target.value); setPage(1) }} className="h-7 rounded-none border border-border bg-background px-2 text-xs outline-none">
              {["all", "completed", "pending", "failed", "refunded"].map((s) => <option key={s} value={s}>{s === "all" ? "All statuses" : s.charAt(0).toUpperCase() + s.slice(1)}</option>)}
            </select>
          </div>
        </div>
        <div className="border border-border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Date</TableHead>
                <TableHead>Course</TableHead>
                <TableHead>Amount</TableHead>
                <TableHead>Status</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {paginated.map((t) => (
                <TableRow key={t.id}>
                  <TableCell className="text-xs text-muted-foreground">{t.createdAt}</TableCell>
                  <TableCell className="text-xs max-w-[200px]">
                    <span className="truncate block">{courseTitle(t.courseId)}</span>
                  </TableCell>
                  <TableCell className="text-xs font-medium">{t.amount > 0 ? `$${t.amount}` : "Free"}</TableCell>
                  <TableCell>
                    <span className={cn("px-1.5 py-0.5 text-xs", statusCls[t.status])}>{t.status}</span>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
        {totalPages > 1 && (
          <div className="mt-3 flex items-center justify-between text-xs text-muted-foreground">
            <span>{filtered.length} transactions · page {page} of {totalPages}</span>
            <div className="flex gap-1">
              <Button variant="outline" size="xs" onClick={() => setPage((p) => Math.max(1, p - 1))} disabled={page === 1}>Previous</Button>
              <Button variant="outline" size="xs" onClick={() => setPage((p) => Math.min(totalPages, p + 1))} disabled={page === totalPages}>Next</Button>
            </div>
          </div>
        )}
      </div>

      {/* Course earnings */}
      <div>
        <p className="mb-3 text-xs font-medium">Earnings by Course</p>
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
          {courseEarnings.map((c) => (
            <div key={c.id} className="border border-border p-3">
              <p className="text-xs font-medium truncate">{c.title}</p>
              <p className="mt-2 text-lg font-semibold">${c.earned.toLocaleString()}</p>
              <p className="text-xs text-muted-foreground">{c.students.toLocaleString()} students · ~${c.avgPerStudent}/student</p>
            </div>
          ))}
        </div>
      </div>

      {/* Payout history */}
      <div>
        <p className="mb-3 text-xs font-medium">Payout History</p>
        <div className="border border-border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Date</TableHead>
                <TableHead>Amount</TableHead>
                <TableHead>Method</TableHead>
                <TableHead>Status</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {payouts.map((p) => (
                <TableRow key={p.id}>
                  <TableCell className="text-xs text-muted-foreground">{p.requestedAt}</TableCell>
                  <TableCell className="text-xs font-medium">${p.amount.toFixed(2)}</TableCell>
                  <TableCell className="text-xs text-muted-foreground">{p.method}</TableCell>
                  <TableCell><span className={cn("px-1.5 py-0.5 text-xs", payoutStatusCls[p.status])}>{p.status}</span></TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      </div>

      {/* Payout dialog */}
      <Dialog open={payoutDialogOpen} onOpenChange={setPayoutDialogOpen}>
        <DialogContent className="max-w-sm">
          <DialogHeader>
            <DialogTitle>Request Payout</DialogTitle>
            <DialogDescription>
              This is a demo. No real payout will be processed.
            </DialogDescription>
          </DialogHeader>
          <div className="py-2">
            <p className="text-xs text-muted-foreground">Available balance</p>
            <p className="text-2xl font-semibold">${summary.available.toFixed(2)}</p>
            <p className="mt-3 text-xs text-muted-foreground border border-border bg-muted/50 p-2">
              ⚠ Demo only — this application does not process real payouts or collect banking information.
            </p>
          </div>
          <DialogFooter>
            <Button variant="outline" size="sm" onClick={() => setPayoutDialogOpen(false)}>Cancel</Button>
            <Button size="sm" onClick={() => { setPayoutDialogOpen(false); toast.success("Payout requested (demo)", { description: "No real payout was initiated." }) }}>
              Confirm (Demo)
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
