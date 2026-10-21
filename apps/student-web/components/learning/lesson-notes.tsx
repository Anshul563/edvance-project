"use client"

import { useState } from "react"
import { HugeiconsIcon } from "@hugeicons/react"
import {
  Note01Icon,
  PencilEdit01Icon,
  TrashIcon,
} from "@hugeicons/core-free-icons"
import { Button } from "@/components/ui/button"
import { Textarea } from "@/components/ui/textarea"
import { addNote, deleteNote, updateNote } from "@/lib/demo-store"
import { useDemoState } from "@/hooks/use-demo-store"
import { formatDate } from "@/lib/format"
import { EmptyState } from "@/components/shared/empty-state"

interface LessonNotesProps {
  courseId: string
  lessonId: string
}

export function LessonNotes({ courseId, lessonId }: LessonNotesProps) {
  const state = useDemoState()
  const [draft, setDraft] = useState("")
  const [editingId, setEditingId] = useState<string | null>(null)
  const [editingText, setEditingText] = useState("")
  const [status, setStatus] = useState("")

  const notes = state.notes.filter(
    (note) => note.courseId === courseId && note.lessonId === lessonId
  )

  function handleAdd() {
    const text = draft.trim()
    if (!text) return
    addNote({ courseId, lessonId, text })
    setDraft("")
    setStatus("Note saved to this device.")
  }

  return (
    <div className="space-y-4">
      <div className="space-y-2">
        <label htmlFor="lesson-note" className="text-xs font-medium">
          Add a note for this lesson
        </label>
        <Textarea
          id="lesson-note"
          value={draft}
          onChange={(event) => setDraft(event.target.value)}
          placeholder="What do you want to remember about this lesson?"
          rows={3}
          maxLength={600}
          className="text-sm md:text-sm"
        />
        <div className="flex items-center justify-between gap-3">
          <p className="text-[11px] text-muted-foreground" role="status">
            {status}
          </p>
          <div className="flex items-center gap-2">
            <span className="text-[11px] text-muted-foreground tabular-nums">
              {draft.length}/600
            </span>
            <Button
              size="sm"
              className="text-sm md:text-sm"
              disabled={draft.trim().length === 0}
              onClick={handleAdd}
            >
              <HugeiconsIcon icon={Note01Icon} strokeWidth={2} />
              Save note
            </Button>
          </div>
        </div>
      </div>

      {notes.length === 0 ? (
        <EmptyState
          title="No notes for this lesson"
          description="Jot down reminders, timestamps, or questions — notes stay in this browser."
          icon={Note01Icon}
          className="py-8"
        />
      ) : (
        <ul className="space-y-3">
          {notes.map((note) => (
            <li key={note.id} className="border border-border bg-muted/30 p-3">
              {editingId === note.id ? (
                <>
                  <Textarea
                    value={editingText}
                    onChange={(event) => setEditingText(event.target.value)}
                    rows={3}
                    className="text-sm md:text-sm"
                    autoFocus
                  />
                  <div className="mt-2 flex justify-end gap-2">
                    <Button
                      variant="ghost"
                      size="sm"
                      className="text-xs md:text-xs"
                      onClick={() => setEditingId(null)}
                    >
                      Cancel
                    </Button>
                    <Button
                      size="sm"
                      className="text-xs md:text-xs"
                      onClick={() => {
                        const text = editingText.trim()
                        if (!text) return
                        updateNote(note.id, text)
                        setEditingId(null)
                        setStatus("Note updated.")
                      }}
                    >
                      Save changes
                    </Button>
                  </div>
                </>
              ) : (
                <>
                  <p className="text-sm leading-relaxed">{note.text}</p>
                  <div className="mt-2 flex items-center justify-between gap-2">
                    <span className="text-[11px] text-muted-foreground">
                      Updated {formatDate(note.updatedAt)}
                    </span>
                    <span className="flex gap-1">
                      <Button
                        variant="ghost"
                        size="icon-xs"
                        aria-label="Edit note"
                        onClick={() => {
                          setEditingId(note.id)
                          setEditingText(note.text)
                          setStatus("")
                        }}
                      >
                        <HugeiconsIcon
                          icon={PencilEdit01Icon}
                          strokeWidth={2}
                        />
                      </Button>
                      <Button
                        variant="ghost"
                        size="icon-xs"
                        aria-label="Delete note"
                        className="text-destructive hover:bg-destructive/10"
                        onClick={() => {
                          deleteNote(note.id)
                          setStatus("Note deleted.")
                        }}
                      >
                        <HugeiconsIcon icon={TrashIcon} strokeWidth={2} />
                      </Button>
                    </span>
                  </div>
                </>
              )}
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
