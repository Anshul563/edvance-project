import type { Lesson, Section } from "@/types"

/** Deterministic 32-bit hash so every build renders identical lesson ids. */
function hashSeed(input: string): number {
  let hash = 2166136261
  for (let index = 0; index < input.length; index += 1) {
    hash ^= input.charCodeAt(index)
    hash = Math.imul(hash, 16777619)
  }
  return hash >>> 0
}

function mulberry32(seed: number): () => number {
  let state = seed
  return () => {
    state = (state + 0x6d2b79f5) | 0
    let t = Math.imul(state ^ (state >>> 15), 1 | state)
    t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296
  }
}

interface SectionTemplate {
  title: string
  lessons: string[]
}

const SECTION_BANKS: Record<string, SectionTemplate[]> = {
  "web-development": [
    {
      title: "Foundations and Setup",
      lessons: [
        "Welcome, course roadmap, and how to use the resources",
        "Setting up your editor, tooling, and project structure",
        "How the web actually works: requests, responses, and the DOM",
      ],
    },
    {
      title: "Semantic Structure and Styling",
      lessons: [
        "Writing semantic, accessible markup",
        "Modern CSS layout with flexbox and grid",
        "Responsive design: breakpoints, fluid type, and images",
        "Design tokens, variables, and consistent spacing",
      ],
    },
    {
      title: "JavaScript in Practice",
      lessons: [
        "Modern syntax: modules, destructuring, and async/await",
        "DOM events, state, and rendering patterns",
        "Working with APIs: fetch, errors, and loading states",
        "Form validation without fighting the browser",
      ],
    },
    {
      title: "Component Architecture",
      lessons: [
        "Thinking in components: props, state, and composition",
        "Reusable UI primitives and styling strategies",
        "Routing and data flow across pages",
        "Performance habits: memoization and lazy loading",
      ],
    },
    {
      title: "Testing, Accessibility, and Quality",
      lessons: [
        "Testing the behavior users care about",
        "Accessibility audits and keyboard-first fixes",
        "Debugging, linting, and code review habits",
      ],
    },
    {
      title: "Ship It",
      lessons: [
        "Production builds, environments, and configuration",
        "Deploying to the web and verifying the release",
        "Monitoring, iterating, and planning version two",
      ],
    },
  ],
  "ui-ux-design": [
    {
      title: "Design Foundations",
      lessons: [
        "Welcome and how this course is structured",
        "Visual hierarchy, typography, and spacing systems",
        "Color theory in interfaces (and accessible contrast)",
        "Grids, layout rhythm, and responsive composition",
      ],
    },
    {
      title: "Understanding Users",
      lessons: [
        "Planning lightweight user research",
        "Interview scripts and empathy mapping",
        "Turning findings into jobs and needs",
        "Personas that don't become shelfware",
      ],
    },
    {
      title: "From Ideas to Flows",
      lessons: [
        "Information architecture and user flows",
        "Sketching and rapid ideation techniques",
        "Wireframing with intent, not decoration",
        "Choosing patterns users already understand",
      ],
    },
    {
      title: "High-Fidelity Design in Figma",
      lessons: [
        "Figma file setup and component thinking",
        "Auto layout, variants, and design tokens",
        "Building an interface that scales",
        "Micro-interactions and motion principles",
      ],
    },
    {
      title: "Validate and Iterate",
      lessons: [
        "Usability testing on a shoestring",
        "Reading session recordings and heatmaps",
        "Prioritizing fixes with a simple impact matrix",
      ],
    },
    {
      title: "Handoff and Portfolio",
      lessons: [
        "Developer handoff: specs that get built right",
        "Documenting a mini design system",
        "Case studies that show your process",
      ],
    },
  ],
  "data-science": [
    {
      title: "Getting Started with Data",
      lessons: [
        "Welcome and the mindset of a data analyst",
        "Setting up Python, Jupyter, and your workspace",
        "Asking questions that data can actually answer",
      ],
    },
    {
      title: "Python for Data Work",
      lessons: [
        "NumPy arrays and vectorized thinking",
        "pandas DataFrames: selecting, filtering, grouping",
        "Cleaning messy data: nulls, types, and duplicates",
        "Combining datasets with joins and merges",
      ],
    },
    {
      title: "Exploratory Analysis",
      lessons: [
        "Distributions, outliers, and summary statistics",
        "Correlation without fooling yourself",
        "Exploratory charts that reveal structure",
        "Documenting findings for non-technical readers",
      ],
    },
    {
      title: "Statistics You Will Actually Use",
      lessons: [
        "Sampling, uncertainty, and confidence intervals",
        "Hypothesis testing with practical examples",
        "Avoiding common analytical traps",
      ],
    },
    {
      title: "Machine Learning Foundations",
      lessons: [
        "Features, labels, and train/test splits",
        "Linear models and classification basics",
        "Evaluating models beyond accuracy",
        "Preventing leakage and overfitting",
      ],
    },
    {
      title: "Tell the Story",
      lessons: [
        "Building charts that make the point",
        "From notebook to polished analysis",
        "Presenting recommendations with confidence",
      ],
    },
  ],
  "mobile-development": [
    {
      title: "Mobile Foundations",
      lessons: [
        "Welcome and what we will build",
        "How mobile platforms differ from the web",
        "Setting up your mobile development environment",
      ],
    },
    {
      title: "Core UI and Navigation",
      lessons: [
        "Layout systems for small screens",
        "Navigation patterns: stacks, tabs, and drawers",
        "Handling touch, gestures, and keyboard avoidance",
        "Theming: light, dark, and system fonts",
      ],
    },
    {
      title: "State and Data",
      lessons: [
        "Managing app state without chaos",
        "Fetching data and handling loading and errors",
        "Caching and offline-first basics",
      ],
    },
    {
      title: "Device Features",
      lessons: [
        "Camera, images, and media pickers",
        "Notifications that respect the user",
        "Storage, settings, and permissions",
      ],
    },
    {
      title: "Polish and Performance",
      lessons: [
        "Smooth animations that stay at 60fps",
        "Reducing startup time and jank",
        "Accessibility on mobile: talkback and contrast",
      ],
    },
    {
      title: "Release",
      lessons: [
        "Testing across devices and screen sizes",
        "App store preparation and review checklists",
        "Crash reporting, analytics, and staged rollouts",
      ],
    },
  ],
  programming: [
    {
      title: "Thinking Like a Programmer",
      lessons: [
        "Welcome and how to get unstuck",
        "Setting up your environment and workflow",
        "Decomposing problems into small steps",
      ],
    },
    {
      title: "Language Fundamentals",
      lessons: [
        "Values, variables, and types",
        "Conditionals, loops, and iteration patterns",
        "Functions, scope, and reuse",
        "Collections: lists, maps, and sets",
      ],
    },
    {
      title: "Data and Algorithms",
      lessons: [
        "Strings, numbers, and common transformations",
        "Searching and sorting by hand and in code",
        "Recursion without the mystery",
        "Complexity: big-O in plain language",
      ],
    },
    {
      title: "Object-Oriented Design",
      lessons: [
        "Classes, objects, and encapsulation",
        "Inheritance vs composition in practice",
        "Interfaces and contracts between modules",
      ],
    },
    {
      title: "Building Real Programs",
      lessons: [
        "Reading and writing clean, idiomatic code",
        "Error handling and defensive programming",
        "Testing your own functions",
        "Version control basics with Git",
      ],
    },
    {
      title: "Practice and Next Steps",
      lessons: [
        "Solving coding-interview style problems",
        "Small project: plan, build, refactor",
        "Where to go from here",
      ],
    },
  ],
  business: [
    {
      title: "Foundations",
      lessons: [
        "Welcome and choosing a worthwhile problem",
        "Opportunity spotting and idea validation",
        "Defining your customer and value proposition",
      ],
    },
    {
      title: "Strategy and Research",
      lessons: [
        "Market sizing you can defend",
        "Competitor analysis without paranoia",
        "Positioning: being different on purpose",
        "Business models and revenue logic",
      ],
    },
    {
      title: "Building the Product Story",
      lessons: [
        "Lean experiments and validation sprints",
        "Pricing psychology and packaging",
        "Messaging that converts",
      ],
    },
    {
      title: "Growth and Marketing",
      lessons: [
        "Channels: choosing where to show up",
        "Content and community that compounds",
        "Retention loops and referral mechanics",
        "Reading your funnel metrics",
      ],
    },
    {
      title: "Operations and Money",
      lessons: [
        "Unit economics in one page",
        "Cash flow, burn, and runway",
        "Hiring your first collaborators",
      ],
    },
    {
      title: "Leading the Venture",
      lessons: [
        "Founder habits and decision journals",
        "Raising (or not raising) capital",
        "Building a culture people stay for",
      ],
    },
  ],
}

const GENERIC_TAIL: SectionTemplate = {
  title: "Capstone Project",
  lessons: [
    "Project briefing and acceptance criteria",
    "Building step by step with instructor guidance",
    "Review, polish, and wrapping up",
  ],
}

const LESSON_TYPES: Lesson["type"][] = [
  "video",
  "video",
  "video",
  "article",
  "video",
  "quiz",
  "video",
]

export function buildSections(
  categorySlug: string,
  seed: string,
  options?: { withCapstone?: boolean }
): Section[] {
  const bank = SECTION_BANKS[categorySlug] ?? SECTION_BANKS["programming"]
  const random = mulberry32(hashSeed(seed))
  const templates = [...bank]
  if (options?.withCapstone !== false) templates.push(GENERIC_TAIL)

  return templates.map((template, sectionIndex) => {
    const sectionId = `${seed}-s${sectionIndex}`
    const lessons: Lesson[] = template.lessons.map((title, lessonIndex) => {
      const lessonId = `${sectionId}-l${lessonIndex}`
      const type =
        lessonIndex === template.lessons.length - 1
          ? LESSON_TYPES[5]
          : LESSON_TYPES[Math.floor(random() * 5)]
      const duration =
        type === "quiz" || type === "assignment"
          ? Math.round(120 + random() * 180)
          : Math.round(330 + random() * 720)
      return {
        id: lessonId,
        title,
        duration,
        type,
        preview: sectionIndex === 0 && lessonIndex < 2,
        content: `In this lesson you will work through “${title}”. Follow along with the downloadable resources, then check the notes tab to capture your own reminders.`,
      }
    })
    return { id: sectionId, title: template.title, lessons }
  })
}

export function totalDuration(sections: Section[]): number {
  return sections.reduce(
    (sum, section) =>
      sum +
      section.lessons.reduce(
        (lessonSum, lesson) => lessonSum + lesson.duration,
        0
      ),
    0
  )
}

export function totalLessons(sections: Section[]): number {
  return sections.reduce((sum, section) => sum + section.lessons.length, 0)
}
