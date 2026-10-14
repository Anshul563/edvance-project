export interface Course {
  id: string
  title: string
  subtitle: string
  description: string
  category: string
  subcategory?: string
  difficulty: "beginner" | "intermediate" | "advanced"
  language: "en" | "es" | "fr" | "de" | "zh" | "ja"
  thumbnail: string
  lessons: Lesson[]
  sections?: Section[]
  students: number
  rating: number
  revenue: number
  status: "draft" | "in-review" | "published" | "rejected" | "archived"
  createdAt: string
  publishedAt?: string
  pricing: "free" | "paid"
  price?: number
  discountPrice?: number
  learningObjectives: string[]
  prerequisites: string[]
  intendedAudience: string
  instructor: string
}

export interface Lesson {
  id: string
  title: string
  description?: string
  type: "video" | "article" | "quiz" | "resource"
  preview: boolean
  videoUrl?: string
  duration?: number // seconds
  order: number
}

export interface CourseFormStep1 {
  title: string
  subtitle: string
  description: string
  category: string
  subcategory?: string
  difficulty: "beginner" | "intermediate" | "advanced"
  language: "en" | "es" | "fr" | "de" | "zh" | "ja"
  learningObjectives: string[]
  prerequisites: string[]
  intendedAudience: string
}

export interface CourseFormStep2 {
  thumbnail: string
  promotionalVideo?: string
}

export interface CourseFormStep3 {
  sections: Section[]
}

export interface Section {
  id: string
  title: string
  description?: string
  lessons: Lesson[]
  order?: number
}

export interface CourseFormStep4 {
  pricing: "free" | "paid"
  price?: number
  discountPrice?: number
}

export interface CourseFormStep5 {
  checklist: string[]
  missing: string[]
}

export interface Student {
  id: string
  name: string
  email: string
  avatar: string
  enrolledCourse: string
  enrolledCourseId: string
  enrollmentDate: string
  progress: number
  lastActivity: string
}

export interface Enrollment {
  id: string
  studentId: string
  courseId: string
  enrolledAt: string
  progress: number
  completedAt?: string
}

export interface Review {
  id: string
  courseId: string
  studentId: string
  rating: number
  title: string
  content: string
  createdAt: string
  creatorReply?: string
}

export interface Transaction {
  id: string
  amount: number
  currency: "USD"
  status: "pending" | "completed" | "failed" | "refunded"
  createdAt: string
  courseId: string
  payoutId?: string
}

export interface Payout {
  id: string
  amount: number
  status: "pending" | "completed" | "failed"
  requestedAt: string
  completedAt?: string
  method: string
}

export interface MediaFile {
  id: string
  name: string
  type: "video" | "image" | "document"
  size: number // bytes
  url: string
  thumbnailUrl?: string
  status: "ready" | "processing" | "failed" | "uploaded"
  uploadedAt: string
  courseId?: string
  duration?: number // seconds for video
}

export interface Notification {
  id: string
  type: "enrollment" | "review" | "earnings" | "system"
  title: string
  message: string
  read: boolean
  createdAt: string
  actionUrl?: string
}

export interface Message {
  id: string
  from: string
  to: string
  subject: string
  content: string
  createdAt: string
  read: boolean
}

export interface Conversation {
  id: string
  participants: string[]
  messages: Message[]
  lastMessageAt: string
  unreadCount: number
}

export interface CommunityPost {
  id: string
  author: string
  title: string
  content: string
  createdAt: string
  likes: number
  comments: number
  commentList?: CommunityComment[]
}

export interface CommunityComment {
  id: string
  postId: string
  author: string
  content: string
  createdAt: string
  likes: number
}

export interface AnalyticsData {
  labels: string[]
  revenue: number[]
  enrollments: number[]
  completionRates: number[]
  engagement: number[]
}

export interface RevenueOverTime {
  date: string
  revenue: number
  enrollments: number
}

export interface CoursePerformance {
  id: string
  title: string
  revenue: number
  students: number
  rating: number
}

export interface CreatorProfile {
  id: string
  name: string
  email: string
  avatar: string
  bio: string
  website?: string
  twitterHandle?: string
  linkedinUrl?: string
  expertise: string[]
  totalStudents: number
  totalCourses: number
  totalRevenue: number
  rating: number
  joinedAt: string
}
