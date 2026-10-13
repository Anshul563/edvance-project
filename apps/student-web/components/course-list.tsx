import { useState, useEffect } from 'react'
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { useNavigate } from "react-router-dom"
import { api, type Course, type CourseSearchFilters } from "@/api-client"

export function useCourseSearch(filters: CourseSearchFilters = {}) {
  const [courses, setCourses] = useState<Course[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const loadCourses = async () => {
    setLoading(true)
    setError(null)
    try {
      const result = await api.request<Course[]>('GET', '/api/v1/search', { body: filters })
      if (result.success) {
        setCourses(result.data || [])
      } else {
        setError(result.error?.message || "Failed to load courses")
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load courses")
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    loadCourses()
  }, [filters])

  return { courses, loading, error, refetch: loadCourses }
}

export function CourseListPanel(filters: CourseSearchFilters = {}) {
  const { courses, loading, error, refetch } = useCourseSearch(filters)

  if (loading) {
    return (
      <div className="py-8 text-center">
        <span className="animate-spin inline-block mr-2">...</span>
        Loading courses
      </div>
    )
  }

  if (error) {
    return (
      <div className="py-8 text-center text-error">
        <p>Error loading courses: {error}</p>
        <Button onClick={refetch} className="mt-2">Retry</Button>
      )
    }
  }

  if (courses.length === 0) {
    return (
      <div className="py-8 text-center text-muted-foreground">
        <p>No courses found matching your criteria.</p>
      </div>
    )
  }

  return (
    <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
      {courses.map((course) => (
        <Card key={course.id} className="h-full">
          <CardHeader>
            <CardTitle>{course.title}</CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-sm truncate-line-clamp-2">{course.description}</p>
            <div className="mt-2 flex items-baseline justify-between">
              <span className="text-sm text-muted-foreground">{course.level}</span>
              <span className="text-sm">
                {course.price > 0 ? `${course.price} ` + (course.currency || "$") : "Free"}
              </span>
            </div>
          </CardContent>
        </Card>
      ))}
    </div>
  )
}
