import type { Transaction, Payout } from "@/src/types/creator"

export const sampleTransactions: Transaction[] = [
  // May 2024
  { id: "t1",  amount: 49,  currency: "USD", status: "completed", createdAt: "2024-05-15", courseId: "c1" },
  { id: "t2",  amount: 79,  currency: "USD", status: "completed", createdAt: "2024-05-14", courseId: "c2" },
  { id: "t3",  amount: 29,  currency: "USD", status: "completed", createdAt: "2024-05-13", courseId: "c3" },
  { id: "t4",  amount: 99,  currency: "USD", status: "completed", createdAt: "2024-05-12", courseId: "c4" },
  { id: "t5",  amount: 59,  currency: "USD", status: "completed", createdAt: "2024-05-11", courseId: "c5" },
  { id: "t6",  amount: 99,  currency: "USD", status: "completed", createdAt: "2024-05-10", courseId: "c4" },
  { id: "t7",  amount: 59,  currency: "USD", status: "completed", createdAt: "2024-05-09", courseId: "c5" },
  { id: "t8",  amount: 0,   currency: "USD", status: "completed", createdAt: "2024-05-08", courseId: "c8" },
  { id: "t9",  amount: 49,  currency: "USD", status: "completed", createdAt: "2024-05-07", courseId: "c1" },
  { id: "t10", amount: 79,  currency: "USD", status: "completed", createdAt: "2024-05-06", courseId: "c2" },
  { id: "t11", amount: 59,  currency: "USD", status: "completed", createdAt: "2024-05-04", courseId: "c5" },
  { id: "t12", amount: 49,  currency: "USD", status: "pending",   createdAt: "2024-05-16", courseId: "c1" },
  // April 2024
  { id: "t13", amount: 129, currency: "USD", status: "completed", createdAt: "2024-04-28", courseId: "c6" },
  { id: "t14", amount: 59,  currency: "USD", status: "completed", createdAt: "2024-04-25", courseId: "c5" },
  { id: "t15", amount: 49,  currency: "USD", status: "completed", createdAt: "2024-04-22", courseId: "c1" },
  { id: "t16", amount: 79,  currency: "USD", status: "completed", createdAt: "2024-04-18", courseId: "c2" },
  { id: "t17", amount: 99,  currency: "USD", status: "refunded",  createdAt: "2024-04-15", courseId: "c4" },
  { id: "t18", amount: 149, currency: "USD", status: "completed", createdAt: "2024-04-12", courseId: "c7" },
  { id: "t19", amount: 59,  currency: "USD", status: "completed", createdAt: "2024-04-08", courseId: "c5" },
  { id: "t20", amount: 29,  currency: "USD", status: "completed", createdAt: "2024-04-05", courseId: "c3" },
  // March 2024
  { id: "t21", amount: 49,  currency: "USD", status: "completed", createdAt: "2024-03-30", courseId: "c1" },
  { id: "t22", amount: 79,  currency: "USD", status: "completed", createdAt: "2024-03-26", courseId: "c2" },
  { id: "t23", amount: 99,  currency: "USD", status: "completed", createdAt: "2024-03-22", courseId: "c4" },
  { id: "t24", amount: 59,  currency: "USD", status: "completed", createdAt: "2024-03-18", courseId: "c5" },
  { id: "t25", amount: 49,  currency: "USD", status: "completed", createdAt: "2024-03-14", courseId: "c1" },
  { id: "t26", amount: 0,   currency: "USD", status: "completed", createdAt: "2024-03-10", courseId: "c8" },
  { id: "t27", amount: 59,  currency: "USD", status: "failed",    createdAt: "2024-03-07", courseId: "c5" },
  // February 2024
  { id: "t28", amount: 49,  currency: "USD", status: "completed", createdAt: "2024-02-28", courseId: "c1" },
  { id: "t29", amount: 79,  currency: "USD", status: "completed", createdAt: "2024-02-22", courseId: "c2" },
  { id: "t30", amount: 59,  currency: "USD", status: "completed", createdAt: "2024-02-18", courseId: "c5" },
  { id: "t31", amount: 99,  currency: "USD", status: "completed", createdAt: "2024-02-14", courseId: "c4" },
  { id: "t32", amount: 49,  currency: "USD", status: "completed", createdAt: "2024-02-10", courseId: "c1" },
  // January 2024
  { id: "t33", amount: 59,  currency: "USD", status: "completed", createdAt: "2024-01-30", courseId: "c5" },
  { id: "t34", amount: 49,  currency: "USD", status: "completed", createdAt: "2024-01-25", courseId: "c1" },
  { id: "t35", amount: 79,  currency: "USD", status: "completed", createdAt: "2024-01-20", courseId: "c2" },
  { id: "t36", amount: 59,  currency: "USD", status: "completed", createdAt: "2024-01-15", courseId: "c5" },
  { id: "t37", amount: 0,   currency: "USD", status: "completed", createdAt: "2024-01-10", courseId: "c8" },
  { id: "t38", amount: 49,  currency: "USD", status: "completed", createdAt: "2024-01-05", courseId: "c1" },
]

export const samplePayouts: Payout[] = [
  { id: "p1", amount: 450.00, status: "completed", requestedAt: "2024-05-01", completedAt: "2024-05-03", method: "Bank Transfer" },
  { id: "p2", amount: 380.50, status: "completed", requestedAt: "2024-04-01", completedAt: "2024-04-03", method: "Bank Transfer" },
  { id: "p3", amount: 520.00, status: "pending",   requestedAt: "2024-05-15", method: "Bank Transfer" },
  { id: "p4", amount: 310.75, status: "completed", requestedAt: "2024-03-01", completedAt: "2024-03-04", method: "Bank Transfer" },
  { id: "p5", amount: 275.00, status: "completed", requestedAt: "2024-02-01", completedAt: "2024-02-05", method: "Bank Transfer" },
  { id: "p6", amount: 195.50, status: "failed",    requestedAt: "2024-01-15", method: "Bank Transfer" },
]

export const sampleEarningsSummary = {
  availableBalance: 1245.50,
  pendingEarnings: 289.75,
  lifetimeEarnings: 58970.00,
  monthlyRevenue: 7230.00,
}
