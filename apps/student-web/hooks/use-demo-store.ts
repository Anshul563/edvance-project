"use client"

import { useEffect, useSyncExternalStore } from "react"
import {
  getServerSnapshot,
  getState,
  hydrate,
  subscribe,
  type DemoState,
} from "@/lib/demo-store"

/**
 * Returns the whole demo state. The reference only changes when state actually
 * changes, so derive slices with `useMemo` instead of adding selectors here.
 *
 * Server renders the deterministic seed; hydration happens once after mount.
 */
export function useDemoState(): DemoState {
  const state = useSyncExternalStore(subscribe, getState, getServerSnapshot)
  useEffect(() => {
    hydrate()
  }, [])
  return state
}
