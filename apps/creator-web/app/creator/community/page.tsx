"use client"

import { useState } from "react"
import { toast } from "sonner"
import { cn } from "cn"
import { HugeiconsIcon } from "@hugeicons/react"
import { Add01Icon, ArrowLeft01Icon, ReplyIcon } from "@hugeicons/core-free-icons"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Textarea } from "@/components/ui/textarea"
import { Label } from "@/components/ui/label"
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter } from "@/components/ui/dialog"
import { demoStore } from "@/src/lib/demo-store"
import type { CommunityPost, CommunityComment } from "@/src/types/creator"

const CATEGORIES = ["General", "Questions", "Announcements", "Feedback"]

const categoryColor: Record<string, string> = {
  General: "bg-blue-100 text-blue-700",
  Questions: "bg-amber-100 text-amber-700",
  Announcements: "bg-green-100 text-green-700",
  Feedback: "bg-purple-100 text-purple-700",
}

function initials(name: string) {
  return name.split(" ").map((n) => n[0]).join("").toUpperCase()
}

export default function CommunityPage() {
  const [posts, setPosts] = useState<CommunityPost[]>(() => demoStore.getCommunityPosts())
  const [selectedPost, setSelectedPost] = useState<CommunityPost | null>(null)
  const [newPostOpen, setNewPostOpen] = useState(false)
  const [newPost, setNewPost] = useState({ title: "", category: "General", content: "" })
  const [commentText, setCommentText] = useState("")

  function createPost() {
    if (!newPost.title.trim() || !newPost.content.trim()) return
    const post: CommunityPost = {
      id: `post-${Date.now()}`,
      author: "Alex Morgan",
      title: newPost.title.trim(),
      content: newPost.content.trim(),
      createdAt: new Date().toISOString().split("T")[0],
      likes: 0,
      comments: 0,
      commentList: [],
    }
    setPosts((ps) => [post, ...ps])
    setNewPost({ title: "", category: "General", content: "" })
    setNewPostOpen(false)
    toast.success("Post created", { description: "Demo — visible only in this session." })
  }

  function addComment() {
    if (!commentText.trim() || !selectedPost) return
    const comment: CommunityComment = {
      id: `c-${Date.now()}`,
      postId: selectedPost.id,
      author: "Alex Morgan",
      content: commentText.trim(),
      createdAt: new Date().toISOString().split("T")[0],
      likes: 0,
    }
    setPosts((ps) => ps.map((p) =>
      p.id === selectedPost.id
        ? { ...p, comments: p.comments + 1, commentList: [...(p.commentList ?? []), comment] }
        : p
    ))
    const updated = posts.find((p) => p.id === selectedPost.id)
    if (updated) setSelectedPost({ ...updated, commentList: [...(updated.commentList ?? []), comment] })
    setCommentText("")
    toast.success("Comment added (demo)")
  }

  function likePost(id: string) {
    setPosts((ps) => ps.map((p) => p.id === id ? { ...p, likes: p.likes + 1 } : p))
  }

  if (selectedPost) {
    const post = posts.find((p) => p.id === selectedPost.id) ?? selectedPost
    return (
      <div className="p-4 sm:p-6 max-w-3xl mx-auto">
        <Button variant="ghost" size="sm" className="mb-4" onClick={() => setSelectedPost(null)}>
          <HugeiconsIcon icon={ArrowLeft01Icon} strokeWidth={2} className="mr-1 h-3.5 w-3.5" />
          Back to community
        </Button>
        <div className="border border-border p-4 space-y-4">
          <div className="flex items-start gap-3">
            <div className="flex h-8 w-8 flex-shrink-0 items-center justify-center bg-primary/10 text-xs font-semibold text-primary">
              {initials(post.author)}
            </div>
            <div className="flex-1">
              <p className="text-sm font-semibold">{post.title}</p>
              <p className="text-xs text-muted-foreground">{post.author} · {post.createdAt}</p>
              <p className="mt-2 text-xs leading-relaxed">{post.content}</p>
            </div>
          </div>
          <div className="flex gap-4 text-xs text-muted-foreground">
            <button onClick={() => likePost(post.id)} className="hover:text-foreground">♡ {post.likes} likes</button>
            <span>💬 {post.comments} comments</span>
          </div>
        </div>

        {/* Comments */}
        <div className="mt-4 space-y-3">
          <p className="text-xs font-medium">{post.comments} Comments</p>
          {(post.commentList ?? []).map((c) => (
            <div key={c.id} className="flex items-start gap-2.5 border-b border-border pb-3">
              <div className="flex h-6 w-6 flex-shrink-0 items-center justify-center bg-muted text-xs font-semibold">
                {initials(c.author)}
              </div>
              <div>
                <p className="text-xs font-medium">{c.author} <span className="font-normal text-muted-foreground">· {c.createdAt}</span></p>
                <p className="mt-0.5 text-xs text-muted-foreground">{c.content}</p>
              </div>
            </div>
          ))}
          <div className="flex gap-2">
            <Input value={commentText} onChange={(e) => setCommentText(e.target.value)} placeholder="Add a comment... (demo)" className="flex-1" onKeyDown={(e) => e.key === "Enter" && addComment()} />
            <Button size="sm" onClick={addComment} disabled={!commentText.trim()}>
              <HugeiconsIcon icon={ReplyIcon} strokeWidth={2} className="h-3.5 w-3.5" />
            </Button>
          </div>
        </div>
      </div>
    )
  }

  return (
    <div className="p-4 sm:p-6 max-w-3xl mx-auto">
      <div className="mb-6 flex items-center justify-between">
        <div>
          <h2 className="text-lg font-semibold">Community</h2>
          <p className="text-xs text-muted-foreground mt-0.5">{posts.length} discussions (demo)</p>
        </div>
        <Button size="sm" onClick={() => setNewPostOpen(true)}>
          <HugeiconsIcon icon={Add01Icon} strokeWidth={2} className="mr-1.5 h-3.5 w-3.5" />
          New Post
        </Button>
      </div>

      <div className="space-y-3">
        {posts.map((post) => (
          <div
            key={post.id}
            className="border border-border p-4 hover:bg-muted/30 transition-colors cursor-pointer"
            onClick={() => setSelectedPost(post)}
          >
            <div className="flex items-start gap-3">
              <div className="flex h-8 w-8 flex-shrink-0 items-center justify-center bg-primary/10 text-xs font-semibold text-primary">
                {initials(post.author)}
              </div>
              <div className="flex-1 min-w-0">
                <div className="flex items-start justify-between gap-2">
                  <p className="text-xs font-semibold">{post.title}</p>
                </div>
                <p className="mt-0.5 text-xs text-muted-foreground">{post.author} · {post.createdAt}</p>
                <p className="mt-1 text-xs text-muted-foreground line-clamp-2">{post.content}</p>
                <div className="mt-2 flex gap-3 text-xs text-muted-foreground">
                  <span>♡ {post.likes}</span>
                  <span>💬 {post.comments}</span>
                </div>
              </div>
            </div>
          </div>
        ))}
      </div>

      {/* New post dialog */}
      <Dialog open={newPostOpen} onOpenChange={setNewPostOpen}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>Create Post</DialogTitle>
            <DialogDescription>Share with the community (demo — visible in this session only)</DialogDescription>
          </DialogHeader>
          <div className="space-y-3 py-2">
            <div>
              <Label className="text-xs font-medium">Title</Label>
              <Input value={newPost.title} onChange={(e) => setNewPost((p) => ({ ...p, title: e.target.value }))} className="mt-1" placeholder="Post title" />
            </div>
            <div>
              <Label className="text-xs font-medium">Category</Label>
              <select value={newPost.category} onChange={(e) => setNewPost((p) => ({ ...p, category: e.target.value }))} className="mt-1 h-8 w-full rounded-none border border-border bg-background px-2 text-xs outline-none focus:ring-1 focus:ring-ring/50">
                {CATEGORIES.map((c) => <option key={c} value={c}>{c}</option>)}
              </select>
            </div>
            <div>
              <Label className="text-xs font-medium">Content</Label>
              <Textarea value={newPost.content} onChange={(e) => setNewPost((p) => ({ ...p, content: e.target.value }))} className="mt-1" rows={4} placeholder="Share your thoughts..." />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" size="sm" onClick={() => setNewPostOpen(false)}>Cancel</Button>
            <Button size="sm" onClick={createPost} disabled={!newPost.title.trim() || !newPost.content.trim()}>Post</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
