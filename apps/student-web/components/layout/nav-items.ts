import type { IconSvgElement } from "@hugeicons/react"
import {
  BookOpen01Icon,
  Certificate01Icon,
  DashboardSquare01Icon,
  GridViewIcon,
  HeartIcon,
  Rocket01Icon,
  Settings01Icon,
  UserIcon,
} from "@hugeicons/core-free-icons"

export interface NavItem {
  href: string
  label: string
  icon: IconSvgElement
  exact?: boolean
}

export const PUBLIC_NAV: NavItem[] = [
  { href: "/courses", label: "Courses", icon: BookOpen01Icon },
  { href: "/become-instructor", label: "Teach on Edvance", icon: Rocket01Icon },
]

export const LEARNER_NAV: NavItem[] = [
  {
    href: "/dashboard",
    label: "Dashboard",
    icon: DashboardSquare01Icon,
    exact: true,
  },
  { href: "/my-learning", label: "My Learning", icon: BookOpen01Icon },
  { href: "/wishlist", label: "Wishlist", icon: HeartIcon },
  { href: "/certificates", label: "Certificates", icon: Certificate01Icon },
  { href: "/notifications", label: "Notifications", icon: GridViewIcon },
  { href: "/profile", label: "Profile", icon: UserIcon },
  { href: "/settings", label: "Settings", icon: Settings01Icon },
]

export function isActivePath(
  href: string,
  pathname: string,
  exact?: boolean
): boolean {
  if (exact) return pathname === href
  return pathname === href || pathname.startsWith(`${href}/`)
}

export const LEARNER_ROOT_PREFIXES = [
  "/dashboard",
  "/my-learning",
  "/learn",
  "/wishlist",
  "/certificates",
  "/notifications",
  "/profile",
  "/settings",
]

export function isLearnerPath(pathname: string): boolean {
  return LEARNER_ROOT_PREFIXES.some(
    (prefix) => pathname === prefix || pathname.startsWith(`${prefix}/`)
  )
}
