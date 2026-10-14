import type { AnalyticsData, RevenueOverTime, CoursePerformance } from "@/src/types/creator"

export const sampleAnalytics: AnalyticsData = {
  labels: ["Week 1", "Week 2", "Week 3", "Week 4", "Week 5", "Week 6", "Week 7"],
  revenue: [1200, 2800, 2400, 3100, 3500, 2900, 2600],
  enrollments: [25, 48, 39, 55, 62, 51, 44],
  completionRates: [55, 62, 58, 65, 68, 61, 57],
  engagement: [1800, 2900, 2400, 3100, 3500, 2900, 2600],
}

// ---------------------------------------------------------------------------
// 7-day revenue (daily granularity, ~May 9-15 2024)
// ---------------------------------------------------------------------------
const revenue7d: RevenueOverTime[] = [
  { date: "May 9",  revenue: 310, enrollments: 7 },
  { date: "May 10", revenue: 280, enrollments: 6 },
  { date: "May 11", revenue: 390, enrollments: 9 },
  { date: "May 12", revenue: 450, enrollments: 11 },
  { date: "May 13", revenue: 310, enrollments: 7 },
  { date: "May 14", revenue: 420, enrollments: 10 },
  { date: "May 15", revenue: 370, enrollments: 8 },
]

// ---------------------------------------------------------------------------
// 30-day revenue (daily granularity, ~April 16 – May 15 2024)
// ---------------------------------------------------------------------------
const revenue30d: RevenueOverTime[] = [
  { date: "Apr 16", revenue: 210, enrollments: 5 },
  { date: "Apr 17", revenue: 340, enrollments: 8 },
  { date: "Apr 18", revenue: 290, enrollments: 7 },
  { date: "Apr 19", revenue: 180, enrollments: 4 },
  { date: "Apr 20", revenue: 260, enrollments: 6 },
  { date: "Apr 21", revenue: 410, enrollments: 10 },
  { date: "Apr 22", revenue: 350, enrollments: 9 },
  { date: "Apr 23", revenue: 220, enrollments: 5 },
  { date: "Apr 24", revenue: 300, enrollments: 7 },
  { date: "Apr 25", revenue: 470, enrollments: 11 },
  { date: "Apr 26", revenue: 390, enrollments: 9 },
  { date: "Apr 27", revenue: 250, enrollments: 6 },
  { date: "Apr 28", revenue: 360, enrollments: 8 },
  { date: "Apr 29", revenue: 190, enrollments: 4 },
  { date: "Apr 30", revenue: 310, enrollments: 7 },
  { date: "May 1",  revenue: 430, enrollments: 10 },
  { date: "May 2",  revenue: 280, enrollments: 6 },
  { date: "May 3",  revenue: 340, enrollments: 8 },
  { date: "May 4",  revenue: 490, enrollments: 12 },
  { date: "May 5",  revenue: 360, enrollments: 9 },
  { date: "May 6",  revenue: 270, enrollments: 6 },
  { date: "May 7",  revenue: 410, enrollments: 10 },
  { date: "May 8",  revenue: 330, enrollments: 8 },
  { date: "May 9",  revenue: 310, enrollments: 7 },
  { date: "May 10", revenue: 280, enrollments: 6 },
  { date: "May 11", revenue: 390, enrollments: 9 },
  { date: "May 12", revenue: 450, enrollments: 11 },
  { date: "May 13", revenue: 310, enrollments: 7 },
  { date: "May 14", revenue: 420, enrollments: 10 },
  { date: "May 15", revenue: 370, enrollments: 8 },
]

// ---------------------------------------------------------------------------
// 90-day revenue (weekly granularity, ~13 weeks)
// ---------------------------------------------------------------------------
const revenue90d: RevenueOverTime[] = [
  { date: "Feb 12", revenue: 1640, enrollments: 38 },
  { date: "Feb 19", revenue: 1820, enrollments: 42 },
  { date: "Feb 26", revenue: 2100, enrollments: 49 },
  { date: "Mar 4",  revenue: 1950, enrollments: 45 },
  { date: "Mar 11", revenue: 2340, enrollments: 54 },
  { date: "Mar 18", revenue: 2580, enrollments: 60 },
  { date: "Mar 25", revenue: 2200, enrollments: 51 },
  { date: "Apr 1",  revenue: 2760, enrollments: 64 },
  { date: "Apr 8",  revenue: 2980, enrollments: 69 },
  { date: "Apr 15", revenue: 3100, enrollments: 72 },
  { date: "Apr 22", revenue: 2850, enrollments: 66 },
  { date: "Apr 29", revenue: 3240, enrollments: 75 },
  { date: "May 6",  revenue: 2530, enrollments: 58 },
]

// ---------------------------------------------------------------------------
// 12-month revenue (monthly granularity, Jun 2023 – May 2024)
// ---------------------------------------------------------------------------
const revenue12m: RevenueOverTime[] = [
  { date: "Jun 2023", revenue: 3200,  enrollments: 74 },
  { date: "Jul 2023", revenue: 2900,  enrollments: 67 },
  { date: "Aug 2023", revenue: 3600,  enrollments: 83 },
  { date: "Sep 2023", revenue: 4100,  enrollments: 95 },
  { date: "Oct 2023", revenue: 4800,  enrollments: 111 },
  { date: "Nov 2023", revenue: 5200,  enrollments: 120 },
  { date: "Dec 2023", revenue: 6100,  enrollments: 141 },
  { date: "Jan 2024", revenue: 7400,  enrollments: 171 },
  { date: "Feb 2024", revenue: 8600,  enrollments: 199 },
  { date: "Mar 2024", revenue: 9800,  enrollments: 227 },
  { date: "Apr 2024", revenue: 11200, enrollments: 259 },
  { date: "May 2024", revenue: 12400, enrollments: 287 },
]

// ---------------------------------------------------------------------------
// Multi-period map (Task 12 requirement)
// ---------------------------------------------------------------------------
export const revenueByPeriod: Record<"7d" | "30d" | "90d" | "12m", RevenueOverTime[]> = {
  "7d":  revenue7d,
  "30d": revenue30d,
  "90d": revenue90d,
  "12m": revenue12m,
}

// Default/legacy export for backwards compatibility
export const sampleRevenueOverTime: RevenueOverTime[] = revenue30d

export const sampleCoursePerformance: CoursePerformance[] = [
  { id: "c1", title: "Python Masterclass",         revenue: 8970,  students: 1248, rating: 4.8 },
  { id: "c2", title: "Data Science Fundamentals",  revenue: 6230,  students: 856,  rating: 4.5 },
  { id: "c3", title: "Creative Writing Essentials",revenue: 3150,  students: 442,  rating: 4.7 },
  { id: "c4", title: "Photography Mastery",        revenue: 5420,  students: 631,  rating: 4.3 },
  { id: "c5", title: "JavaScript Bootcamp",        revenue: 11250, students: 1532, rating: 4.9 },
  { id: "c6", title: "UX Design Fundamentals",     revenue: 4890,  students: 378,  rating: 4.6 },
  { id: "c7", title: "Marketing Analytics",        revenue: 2850,  students: 210,  rating: 4.2 },
  { id: "c8", title: "Productivity Masterclass",   revenue: 7230,  students: 987,  rating: 4.5 },
]
