import type { Category } from "@/types"

export const categories: Category[] = [
  {
    slug: "web-development",
    title: "Web Development",
    description:
      "Build modern websites and web applications with HTML, CSS, JavaScript, and today's most wanted frameworks.",
    icon: "code",
    thumbnail: "/thumbnails/web-development.svg",
  },
  {
    slug: "ui-ux-design",
    title: "UI/UX Design",
    description:
      "Design intuitive, beautiful product experiences — from research and wireframes to polished design systems.",
    icon: "palette",
    thumbnail: "/thumbnails/ui-ux-design.svg",
  },
  {
    slug: "data-science",
    title: "Data Science",
    description:
      "Analyze data, build models, and communicate insights with Python, SQL, and machine learning.",
    icon: "chart",
    thumbnail: "/thumbnails/data-science.svg",
  },
  {
    slug: "mobile-development",
    title: "Mobile Development",
    description:
      "Ship native-quality iOS and Android apps with modern mobile frameworks and practical project work.",
    icon: "mobile",
    thumbnail: "/thumbnails/mobile-development.svg",
  },
  {
    slug: "programming",
    title: "Programming",
    description:
      "Master core programming thinking with Python, Java, and problem-solving fundamentals that transfer everywhere.",
    icon: "terminal",
    thumbnail: "/thumbnails/programming.svg",
  },
  {
    slug: "business",
    title: "Business",
    description:
      "Start, grow, and operate ventures — product strategy, marketing, finance, and leadership skills.",
    icon: "briefcase",
    thumbnail: "/thumbnails/business.svg",
  },
]

export function getCategory(slug: string): Category | undefined {
  return categories.find((category) => category.slug === slug)
}
