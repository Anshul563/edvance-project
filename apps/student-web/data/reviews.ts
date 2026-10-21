import type { Review } from "@/types"
import { courses, getCourseById } from "./courses"

const REVIEWER_NAMES = [
  "Jordan Lee",
  "Aisha Rahman",
  "Marco Silva",
  "Emily Novak",
  "Kenji Sato",
  "Fatima Zahra",
  "Lucas Meyer",
  "Grace Adeyemi",
  "Pavel Novak",
  "Sara Haddad",
  "Diego Ramos",
  "Nina Kowalski",
  "Omar Farouk",
  "Chloe Bennett",
  "Ravi Menon",
  "Isabella Costa",
  "Tom Becker",
  "Yuki Tanaka",
  "Leila Haddad",
  "Andre Dubois",
  "Mia Johansson",
  "Samuel Osei",
  "Vera Klein",
  "Hassan Ali",
]

const TEXT_BANKS: Record<string, string[]> = {
  "web-development": [
    "The project structure finally made things click for me. I rebuilt my side project the next weekend and it already feels more maintainable.",
    "Clear pacing and honest about trade-offs. The debugging lesson alone saved me a full day at work.",
    "I came in able to copy tutorials and left able to design my own features. The exercises are the real course.",
    "Sections are the right length and the resources tab has everything referenced in the videos.",
    "Watched at 1.25x on weekdays. The recap at the end of each section is perfect for revision.",
    "Answered a question I had for months — why my data fetching kept duplicating requests. Straightforward fix, well explained.",
  ],
  "ui-ux-design": [
    "The before/after critiques are gold. I started noticing the same mistakes in apps I use daily.",
    "Practical without being shallow. My Figma files are cleaner and handoff arguments have disappeared.",
    "Finally a design course that talks about accessibility early instead of as an appendix.",
    "I applied the hierarchy lesson to our dashboard and stakeholders noticed within a week.",
    "Great balance of theory and doing. The capstone pushed my portfolio up a level.",
    "Short, focused lessons — easy to fit around a full-time job.",
  ],
  "data-science": [
    "The messy dataset exercises mirror my actual job. No cleaned-and-tidy CSVs here.",
    "Statistics usually makes me nervous; the worked examples made interpretation feel obvious.",
    "Went from copying notebooks to explaining my analysis in meetings without panic.",
    "The section on avoiding leakage probably saved me from shipping a broken model.",
    "Good pace for people with a bit of Python. I rewatched the groupby lesson three times — worth it.",
    "Used the chart guidance for a quarterly report and it was the clearest slide in the deck.",
  ],
  "mobile-development": [
    "Shipped my first app to both stores using the checklist in the release module. Review passed in one shot.",
    "The performance module fixed a stutter I had lived with for months.",
    "Explains native quirks instead of pretending one codebase means zero platform work.",
    "Offline sync design was the part I needed most — the conflict examples were realistic.",
    "Clear, unfussy teaching. Built the course project on the train during my commute.",
    "Great for returning to after a break; the section summaries bring you back fast.",
  ],
  programming: [
    "The visual explanations of the structures finally replaced my memorized definitions with real understanding.",
    "Wish I had this before my first job. The debugging methodology is now muscle memory.",
    "Exercises build on each other nicely — never felt like busywork.",
    "Passed my interview loop with noticeably more composure. The practice problems match the format.",
    "Hannah's way of walking through a wrong approach before the right one sticks with me.",
    "Short lessons, real feedback loops. I actually finished a course for once.",
  ],
  business: [
    "Cut three features from our roadmap after the validation module. Best money our team spent this year.",
    "No hype, no frameworks worship — just experiments with clear readouts.",
    "The financial model template is what I opened the course for; the rest was a pleasant bonus.",
    "Watched with my co-founder and we finally agreed on how to read our funnel metrics.",
    "Concise and evidence-based. The interview script has been reused in every customer call since.",
    "Helped me kill an idea early and start a better one. That is a win.",
  ],
}

const NEGATIVE_TEXTS = [
  "Solid content, but a few sections move faster than I would like when I was completely new to the topic.",
  "Good course overall — I would have appreciated a couple more advanced exercises near the end.",
]

function hash(input: string): number {
  let value = 2166136261
  for (let index = 0; index < input.length; index += 1) {
    value ^= input.charCodeAt(index)
    value = Math.imul(value, 16777619)
  }
  return value >>> 0
}

function pick<T>(pool: T[], seed: number, offset: number): T {
  return pool[(seed + offset * 7919) % pool.length]
}

function daysAgo(days: number): string {
  const date = new Date(Date.UTC(2026, 1, 20))
  date.setUTCDate(date.getUTCDate() - days)
  return date.toISOString().slice(0, 10)
}

const cache = new Map<string, Review[]>()

/** Deterministic per-course reviews: same output on every render and build. */
export function getReviewsForCourse(courseId: string, limit = 6): Review[] {
  const cached = cache.get(courseId)
  if (cached) return cached.slice(0, limit)

  const courseItem = getCourseById(courseId)
  if (!courseItem) return []

  const seed = hash(courseId)
  const bank = TEXT_BANKS[courseItem.categorySlug] ?? TEXT_BANKS["programming"]
  const count = Math.min(limit, 6)
  const reviews: Review[] = []

  for (let index = 0; index < count; index += 1) {
    const isLukewarm = index === count - 1 && count >= 5
    reviews.push({
      id: `${courseId}-rev-${index}`,
      courseId,
      authorName: pick(REVIEWER_NAMES, seed, index + 1),
      rating: isLukewarm ? 3 : seed % 5 === index % 5 ? 4 : 5,
      date: daysAgo((seed % 90) + index * 17 + 6),
      text: isLukewarm
        ? pick(NEGATIVE_TEXTS, seed, index)
        : pick(bank, seed, index + 1),
    })
  }

  cache.set(courseId, reviews)
  return reviews.slice(0, limit)
}

export function getFeaturedReviews(
  limit = 6
): { review: Review; courseTitle: string }[] {
  const featured: { review: Review; courseTitle: string }[] = []
  const step = Math.max(1, Math.floor(courses.length / limit))
  for (
    let index = 0;
    featured.length < limit && index < courses.length;
    index += step
  ) {
    const courseItem = courses[index]
    const review = getReviewsForCourse(courseItem.id, 1)[0]
    if (review) featured.push({ review, courseTitle: courseItem.title })
  }
  return featured
}
