import api, { type ApiResponse } from '@/api-client'

// Helper types for course-related API responses
export interface Course {
  id: string
  title: string
  description: string
  level: string
  price: number
  isPublished: boolean
}

export interface CourseLesson {
  id: string
  title: string
  description: string
  order: number
}

export interface CourseSearchFilters {
  q?: string
  category?: string
  level?: string
}

// Course API functions
export async function searchCourses(filters: CourseSearchFilters = {}): Promise<ApiResponse<Course[]>> {
  return api.request<Course[]>('GET', '/api/v1/search', { body: filters })
}

export async function getCourse(courseId: string): Promise<ApiResponse<{
  id: string
  title: string
  description: string
  level: string
  lessons: CourseLesson[]
}>> {
  return api.request(`/api/v1/courses/${courseId}`)
}

// Auth API functions
export async function login(email: string, password: string) {
  return api.login(email, password)
}

export async function logout() {
  return api.logout()
}

// Export the api instance for direct use
export { api }
