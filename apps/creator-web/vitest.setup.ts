import "@testing-library/jest-dom/vitest"
import { cleanup } from "@testing-library/react"
import { afterEach, vi } from "vitest"

// vitest globals are disabled, so Testing Library cannot register its
// own afterEach cleanup — do it explicitly to avoid DOM accumulation.
afterEach(() => {
  cleanup()
})

// jsdom does not implement media playback; silence the not-implemented
// errors so player tests can drive state through DOM events instead.
vi.spyOn(HTMLMediaElement.prototype, "play").mockImplementation(function (
  this: HTMLMediaElement,
) {
  return Promise.resolve()
})
vi.spyOn(HTMLMediaElement.prototype, "pause").mockImplementation(function (
  this: HTMLMediaElement,
) {})
vi.spyOn(HTMLMediaElement.prototype, "load").mockImplementation(function (
  this: HTMLMediaElement,
) {})
