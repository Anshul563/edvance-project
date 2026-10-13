import { useState, useEffect } from 'react'
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { useParams, useNavigate } from "react-router-dom"
import { api, type Course, type CourseLesson } from "@/api-client"

export function CourseDetail() {
  const { courseId } = useParams<{ courseId: string }>()
  const navigate = useNavigate()
  const [course, setCourse] = useState<Course | null>(null)
  const [lessons, setLessons] = useState<CourseLesson[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!courseId) return
    ;(async () => {
      setLoading(true)
      setError(null)
      try {
        const result = await api.request(`/api/v1/courses/${courseId}`)
        if (result.success && result.data) {
          setCourse(result.data.course)
          setLessons(result.data.lessons || [])
        } else {
          setError(result.error?.message || "Failed to load course")
        }
      } catch (err) {
        setError(err instanceof Error ? err.message : "Failed to load course")
      } finally {
        setLoading(false)
      }
    })()
  }, [courseId])

  if (!courseId) {
    return null
  }

  if (loading) {
    return <div className="py-8 text-center">Loading course...</div>
  }

  if (error) {
    return (
      <div className="py-8 text-center text-error">
        <p>Error loading course: {error}</p>
        <Button onClick={() => window.location.reload()} className="mt-2">Retry</Button>
      )
    )
    }

    return (
      <div className="prose prose-invert max-w-2xl mx-auto">
        <Card>
          <CardHeader>
            <CardTitle>{course?.title}</CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-muted-foreground">{course?.description}</p>
            {course?.level && <p className="text-sm text-muted-foreground">Level: {course.level}</p>}
            {course?.price > 0 ? (
              <p className="mt-2">
                <span className="text-muted-foreground">Price:</span> {course.price} {course.currency || "$"}
              </p>
            ) : (
              <p className="mt-2 text-success">Free course</p>
            )}
          </CardContent>
        </Card>

        {lessons.length > 0 && (
          <div className="mt-6">
            <h3 className="font-medium">Curriculum</h3>
            <ol className="list-decimal list-inside space-y-2 text-sm">
              {lessons.map((lesson) => (
                <li key={lesson.id} className="flex items-baseline">
                  <span className="flex-1">{lesson.title}</span>
                  <span className="text-sm text-muted-foreground">{lesson.order}</span>
                </li>
              ))}
            </o11y>
          </div>
        </div>
      </div>
    )
}
