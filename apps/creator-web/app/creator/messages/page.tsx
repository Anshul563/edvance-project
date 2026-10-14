"use client"

import { useState } from "react"
import { toast } from "sonner"
import { cn } from "cn"
import { HugeiconsIcon } from "@hugeicons/react"
import { ArrowLeft01Icon, SendIcon } from "@hugeicons/core-free-icons"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { SearchFilter } from "@/components/search-filter"
import { demoStore } from "@/src/lib/demo-store"
import type { Conversation, Message } from "@/src/types/creator"

function initials(name: string) {
  return name.split(" ").map((n) => n[0]).join("").toUpperCase()
}

export default function MessagesPage() {
  const [conversations, setConversations] = useState<Conversation[]>(() => demoStore.getConversations())
  const [selected, setSelected] = useState<Conversation | null>(null)
  const [reply, setReply] = useState("")
  const [search, setSearch] = useState("")

  const filtered = search
    ? conversations.filter((c) => c.participants.some((p) => p.toLowerCase().includes(search.toLowerCase())))
    : conversations

  function sendReply() {
    if (!reply.trim() || !selected) return
    const msg: Message = {
      id: `m-${Date.now()}`,
      from: "Alex Morgan",
      to: selected.participants.find((p) => p !== "Alex Morgan") ?? "",
      subject: "Re: " + (selected.messages[0]?.subject ?? ""),
      content: reply.trim(),
      createdAt: new Date().toISOString().split("T")[0],
      read: true,
    }
    const updated = conversations.map((c) =>
      c.id === selected.id
        ? { ...c, messages: [...c.messages, msg], lastMessageAt: msg.createdAt, unreadCount: 0 }
        : c
    )
    setConversations(updated)
    setSelected(updated.find((c) => c.id === selected.id) ?? null)
    setReply("")
    toast.success("Reply sent", { description: "Demo — message not delivered to the student." })
  }

  const otherParticipant = (c: Conversation) => c.participants.find((p) => p !== "Alex Morgan") ?? ""
  const lastMsg = (c: Conversation) => c.messages[c.messages.length - 1]

  return (
    <div className="flex h-[calc(100vh-48px)] divide-x divide-border">
      {/* Conversation list */}
      <div className={cn("flex flex-col border-r border-border", selected ? "hidden md:flex w-72" : "flex flex-1 md:w-72 md:flex-none")}>
        <div className="border-b border-border p-3">
          <h2 className="mb-2 text-sm font-semibold">Messages</h2>
          <SearchFilter placeholder="Search conversations..." onChange={setSearch} />
        </div>
        <div className="flex-1 overflow-y-auto divide-y divide-border">
          {filtered.length === 0 ? (
            <p className="p-4 text-xs text-muted-foreground">No conversations</p>
          ) : filtered.map((c) => {
            const other = otherParticipant(c)
            const last = lastMsg(c)
            const isActive = selected?.id === c.id
            return (
              <button
                key={c.id}
                className={cn("w-full flex items-start gap-2.5 p-3 text-left hover:bg-muted/50 transition-colors", isActive && "bg-muted")}
                onClick={() => { setSelected(c); setConversations((cs) => cs.map((x) => x.id === c.id ? { ...x, unreadCount: 0 } : x)) }}
              >
                <div className="mt-0.5 flex h-8 w-8 flex-shrink-0 items-center justify-center bg-primary/10 text-xs font-semibold text-primary">
                  {initials(other)}
                </div>
                <div className="min-w-0 flex-1">
                  <div className="flex items-center justify-between gap-1">
                    <span className={cn("text-xs", c.unreadCount > 0 ? "font-semibold" : "font-medium")}>{other}</span>
                    <span className="text-xs text-muted-foreground flex-shrink-0">{c.lastMessageAt}</span>
                  </div>
                  <p className="mt-0.5 truncate text-xs text-muted-foreground">{last?.content}</p>
                  {c.unreadCount > 0 && (
                    <span className="mt-1 inline-flex h-4 w-4 items-center justify-center bg-primary text-primary-foreground text-[10px] rounded-full">
                      {c.unreadCount}
                    </span>
                  )}
                </div>
              </button>
            )
          })}
        </div>
      </div>

      {/* Message thread */}
      {selected ? (
        <div className="flex flex-1 flex-col min-w-0">
          {/* Thread header */}
          <div className="flex items-center gap-2 border-b border-border px-4 py-3">
            <Button variant="ghost" size="icon-sm" className="md:hidden" onClick={() => setSelected(null)}>
              <HugeiconsIcon icon={ArrowLeft01Icon} strokeWidth={2} className="h-4 w-4" />
            </Button>
            <div className="flex h-8 w-8 items-center justify-center bg-primary/10 text-xs font-semibold text-primary">
              {initials(otherParticipant(selected))}
            </div>
            <div>
              <p className="text-xs font-medium">{otherParticipant(selected)}</p>
              <p className="text-xs text-muted-foreground">Student</p>
            </div>
          </div>

          {/* Messages */}
          <div className="flex-1 overflow-y-auto p-4 space-y-3">
            {selected.messages.map((msg) => {
              const isMe = msg.from === "Alex Morgan"
              return (
                <div key={msg.id} className={cn("flex", isMe ? "justify-end" : "justify-start")}>
                  <div className={cn("max-w-[70%] rounded-none px-3 py-2", isMe ? "bg-primary text-primary-foreground" : "bg-muted")}>
                    <p className="text-xs leading-relaxed">{msg.content}</p>
                    <p className={cn("mt-1 text-[10px]", isMe ? "text-primary-foreground/70" : "text-muted-foreground")}>{msg.createdAt}</p>
                  </div>
                </div>
              )
            })}
          </div>

          {/* Reply input */}
          <div className="border-t border-border p-3 flex gap-2">
            <Input
              value={reply}
              onChange={(e) => setReply(e.target.value)}
              placeholder="Type a reply… (demo)"
              onKeyDown={(e) => e.key === "Enter" && !e.shiftKey && sendReply()}
              className="flex-1"
            />
            <Button size="sm" onClick={sendReply} disabled={!reply.trim()}>
              <HugeiconsIcon icon={SendIcon} strokeWidth={2} className="h-3.5 w-3.5" />
            </Button>
          </div>
        </div>
      ) : (
        <div className="hidden md:flex flex-1 items-center justify-center">
          <p className="text-xs text-muted-foreground">Select a conversation to view</p>
        </div>
      )}
    </div>
  )
}
