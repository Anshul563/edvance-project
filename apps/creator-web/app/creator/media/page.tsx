"use client"

import { useState, useMemo } from "react"
import { toast } from "sonner"
import { cn } from "cn"
import { HugeiconsIcon } from "@hugeicons/react"
import {
  GridViewIcon,
  ListViewIcon,
  FileUploadIcon,
  Delete01Icon,
  EyeIcon,
  FilterIcon,
  Search01Icon,
} from "@hugeicons/core-free-icons"
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from "@/components/ui/table"
import { Button } from "@/components/ui/button"
import { SearchFilter } from "@/components/search-filter"
import { ConfirmationDialog } from "@/components/confirmation-dialog"
import { EmptyState } from "@/components/empty-state"
import { demoStore } from "@/src/lib/demo-store"
import type { MediaFile } from "@/src/types/creator"
import {
  Sheet, SheetContent, SheetHeader, SheetTitle, SheetDescription,
} from "@/components/ui/sheet"

function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(0)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}

function formatDuration(s?: number): string {
  if (!s) return "—"
  const m = Math.floor(s / 60), sec = s % 60
  return `${m}:${String(sec).padStart(2, "0")}`
}

const statusCls: Record<MediaFile["status"], string> = {
  ready: "bg-green-100 text-green-700",
  processing: "bg-yellow-100 text-yellow-700",
  failed: "bg-red-100 text-red-700",
  uploaded: "bg-blue-100 text-blue-700",
}

const typeCls: Record<MediaFile["type"], string> = {
  video: "bg-violet-100 text-violet-700",
  image: "bg-blue-100 text-blue-700",
  document: "bg-gray-100 text-gray-600",
}

export default function MediaPage() {
  const [files, setFiles] = useState<MediaFile[]>(() => demoStore.getMediaFiles())
  const [view, setView] = useState<"grid" | "list">("grid")
  const [search, setSearch] = useState("")
  const [typeFilter, setTypeFilter] = useState<MediaFile["type"] | "all">("all")
  const [selected, setSelected] = useState<MediaFile | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<string | null>(null)

  const filtered = useMemo(() => {
    let list = files
    if (search) list = list.filter((f) => f.name.toLowerCase().includes(search.toLowerCase()))
    if (typeFilter !== "all") list = list.filter((f) => f.type === typeFilter)
    return list
  }, [files, search, typeFilter])

  function handleDelete(id: string) {
    setFiles((fs) => fs.filter((f) => f.id !== id))
    if (selected?.id === id) setSelected(null)
    toast.success("File deleted")
  }

  return (
    <div className="p-4 sm:p-6 max-w-7xl mx-auto">
      <div className="mb-6 flex items-center justify-between">
        <div>
          <h2 className="text-lg font-semibold">Media Library</h2>
          <p className="text-xs text-muted-foreground mt-0.5">{files.length} files</p>
        </div>
        <Button size="sm" onClick={() => toast.info("Demo", { description: "File upload is simulated in this demo." })}>
          <HugeiconsIcon icon={FileUploadIcon} strokeWidth={2} className="mr-1.5 h-3.5 w-3.5" />
          Upload Media
        </Button>
      </div>

      {/* Toolbar */}
      <div className="mb-4 flex flex-wrap items-center gap-2">
        <SearchFilter placeholder="Search files..." onChange={setSearch} className="w-48" />
        <div className="flex gap-1">
          {(["all", "video", "image", "document"] as const).map((t) => (
            <button
              key={t}
              onClick={() => setTypeFilter(t)}
              className={cn("px-2.5 py-1 text-xs border transition-colors", typeFilter === t ? "border-primary bg-primary text-primary-foreground" : "border-border hover:bg-muted")}
            >
              {t === "all" ? "All" : t.charAt(0).toUpperCase() + t.slice(1) + "s"}
            </button>
          ))}
        </div>
        <div className="ml-auto flex gap-1">
          <Button variant={view === "grid" ? "secondary" : "ghost"} size="icon-sm" onClick={() => setView("grid")} aria-label="Grid view">
            <HugeiconsIcon icon={GridViewIcon} strokeWidth={2} className="h-3.5 w-3.5" />
          </Button>
          <Button variant={view === "list" ? "secondary" : "ghost"} size="icon-sm" onClick={() => setView("list")} aria-label="List view">
            <HugeiconsIcon icon={ListViewIcon} strokeWidth={2} className="h-3.5 w-3.5" />
          </Button>
        </div>
      </div>

      {filtered.length === 0 ? (
        <EmptyState title="No media files found" description="Try adjusting your search or upload some files." />
      ) : view === "grid" ? (
        <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5">
          {filtered.map((file) => (
            <div key={file.id} className="group border border-border flex flex-col">
              <div
                className={cn("flex h-28 items-center justify-center text-2xl font-bold cursor-pointer", typeCls[file.type])}
                onClick={() => setSelected(file)}
              >
                {file.type === "video" ? "▶" : file.type === "image" ? "🖼" : "📄"}
              </div>
              <div className="flex flex-col gap-1 p-2">
                <p className="truncate text-xs font-medium">{file.name}</p>
                <div className="flex items-center justify-between">
                  <span className={cn("rounded-none px-1.5 py-0.5 text-xs", statusCls[file.status])}>{file.status}</span>
                  <span className="text-xs text-muted-foreground">{formatSize(file.size)}</span>
                </div>
                <div className="flex gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
                  <Button size="icon-xs" variant="ghost" onClick={() => setSelected(file)} aria-label="View">
                    <HugeiconsIcon icon={EyeIcon} strokeWidth={2} className="h-3 w-3" />
                  </Button>
                  <Button size="icon-xs" variant="ghost" onClick={() => setDeleteTarget(file.id)} aria-label="Delete">
                    <HugeiconsIcon icon={Delete01Icon} strokeWidth={2} className="h-3 w-3 text-destructive" />
                  </Button>
                </div>
              </div>
            </div>
          ))}
        </div>
      ) : (
        <div className="border border-border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>Type</TableHead>
                <TableHead>Size</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Uploaded</TableHead>
                <TableHead className="w-20" />
              </TableRow>
            </TableHeader>
            <TableBody>
              {filtered.map((file) => (
                <TableRow key={file.id}>
                  <TableCell className="font-medium text-xs">{file.name}</TableCell>
                  <TableCell><span className={cn("px-1.5 py-0.5 text-xs", typeCls[file.type])}>{file.type}</span></TableCell>
                  <TableCell className="text-xs text-muted-foreground">{formatSize(file.size)}</TableCell>
                  <TableCell><span className={cn("px-1.5 py-0.5 text-xs", statusCls[file.status])}>{file.status}</span></TableCell>
                  <TableCell className="text-xs text-muted-foreground">{file.uploadedAt}</TableCell>
                  <TableCell>
                    <div className="flex gap-1">
                      <Button size="icon-xs" variant="ghost" onClick={() => setSelected(file)}><HugeiconsIcon icon={EyeIcon} strokeWidth={2} className="h-3.5 w-3.5" /></Button>
                      <Button size="icon-xs" variant="ghost" onClick={() => setDeleteTarget(file.id)}><HugeiconsIcon icon={Delete01Icon} strokeWidth={2} className="h-3.5 w-3.5 text-destructive" /></Button>
                    </div>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}

      {/* Detail sheet */}
      <Sheet open={!!selected} onOpenChange={(o) => !o && setSelected(null)}>
        <SheetContent side="right" className="w-80">
          <SheetHeader>
            <SheetTitle>{selected?.name}</SheetTitle>
            <SheetDescription>File details</SheetDescription>
          </SheetHeader>
          {selected && (
            <div className="mt-4 space-y-4">
              <div className={cn("flex h-36 items-center justify-center text-4xl", typeCls[selected.type])}>
                {selected.type === "video" ? "▶" : selected.type === "image" ? "🖼" : "📄"}
              </div>
              <dl className="space-y-2 text-xs">
                {[
                  ["Name", selected.name],
                  ["Type", selected.type],
                  ["Size", formatSize(selected.size)],
                  ["Status", selected.status],
                  ["Uploaded", selected.uploadedAt],
                  ...(selected.duration ? [["Duration", formatDuration(selected.duration)]] : []),
                  ...(selected.courseId ? [["Course", selected.courseId]] : []),
                ].map(([k, v]) => (
                  <div key={k} className="flex gap-2">
                    <dt className="w-20 flex-shrink-0 text-muted-foreground">{k}</dt>
                    <dd className="flex-1 break-all">{v}</dd>
                  </div>
                ))}
              </dl>
              <Button
                size="sm"
                variant="destructive"
                className="w-full"
                onClick={() => { if (selected) setDeleteTarget(selected.id) }}
              >
                <HugeiconsIcon icon={Delete01Icon} strokeWidth={2} className="mr-1.5 h-3.5 w-3.5" />
                Delete File
              </Button>
            </div>
          )}
        </SheetContent>
      </Sheet>

      <ConfirmationDialog
        open={!!deleteTarget}
        onOpenChange={(o) => !o && setDeleteTarget(null)}
        title="Delete file?"
        description="This will permanently remove the file from your media library."
        confirmLabel="Delete"
        onConfirm={() => { if (deleteTarget) handleDelete(deleteTarget) }}
      />
    </div>
  )
}
