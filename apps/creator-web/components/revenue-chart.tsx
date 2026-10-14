"use client"

import { AreaChart, Area, XAxis, YAxis, CartesianGrid } from "recharts"
import {
  ChartContainer,
  ChartTooltip,
  ChartTooltipContent,
  type ChartConfig,
} from "@/components/ui/chart"

interface DataPoint {
  name: string
  uv: number
  pv?: number
}

interface RevenueChartProps {
  data: DataPoint[]
  type?: "revenue" | "enrollments"
  className?: string
}

const revenueConfig: ChartConfig = {
  uv: { label: "Revenue ($)", color: "hsl(262, 83%, 58%)" },
  pv: { label: "Prev Period ($)", color: "hsl(262, 40%, 75%)" },
}

const enrollmentConfig: ChartConfig = {
  uv: { label: "Enrollments", color: "hsl(210, 80%, 55%)" },
  pv: { label: "Prev Period", color: "hsl(210, 40%, 75%)" },
}

export function RevenueChart({ data, type = "revenue", className }: RevenueChartProps) {
  const config = type === "revenue" ? revenueConfig : enrollmentConfig

  return (
    <div className={className}>
      <p className="mb-2 text-xs font-medium text-muted-foreground">
        {type === "revenue" ? "Revenue over time" : "Enrollment trend"}
      </p>
      <ChartContainer config={config} className="h-[180px] w-full">
        <AreaChart data={data} margin={{ top: 4, right: 4, bottom: 0, left: 0 }}>
          <CartesianGrid strokeDasharray="3 3" vertical={false} />
          <XAxis dataKey="name" tick={{ fontSize: 10 }} tickLine={false} axisLine={false} />
          <YAxis tick={{ fontSize: 10 }} tickLine={false} axisLine={false} width={36} />
          <ChartTooltip content={<ChartTooltipContent />} />
          {data[0]?.pv !== undefined && (
            <Area
              type="monotone"
              dataKey="pv"
              stroke="var(--color-pv)"
              fill="var(--color-pv)"
              fillOpacity={0.1}
              strokeWidth={1.5}
            />
          )}
          <Area
            type="monotone"
            dataKey="uv"
            stroke="var(--color-uv)"
            fill="var(--color-uv)"
            fillOpacity={0.15}
            strokeWidth={2}
          />
        </AreaChart>
      </ChartContainer>
    </div>
  )
}
