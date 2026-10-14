"use client"

import { useState } from "react"
import { toast } from "sonner"
import { cn } from "cn"
import { HugeiconsIcon } from "@hugeicons/react"
import { Add01Icon, ArrowRight01Icon } from "@hugeicons/core-free-icons"
import { Button } from "@/components/ui/button"
import { Textarea } from "@/components/ui/textarea"
import { Label } from "@/components/ui/label"

const FAQS = [
  {
    q: "How do I publish a course?",
    a: "To publish a course, complete all required fields (title, description, thumbnail, and at least one lesson), then go to the Publishing tab in the Course Editor and click 'Publish'. Your course must first pass a review if your account is in review mode.",
  },
  {
    q: "How are earnings calculated?",
    a: "You earn 70% of each sale after Edvance's 30% platform fee. Coupon discounts and refunds reduce your net earnings. Earnings are held for 7 days before becoming available for payout.",
  },
  {
    q: "What video formats are supported?",
    a: "We support MP4 (H.264), MOV, and AVI formats. Maximum file size is 2GB per video. We recommend 1080p (1920×1080) at 30fps for the best quality. Videos are transcoded to multiple quality levels automatically.",
  },
  {
    q: "How do I respond to student reviews?",
    a: "Go to the Reviews page, find the review you want to reply to, and click the 'Reply' button. Your reply will be publicly visible under the student's review. This is demo-only and replies are not delivered to real students.",
  },
  {
    q: "What are the platform fees?",
    a: "Edvance charges 30% on all paid course sales. Free courses have no fees. Promotional coupon sales are calculated on the coupon price. There are no listing fees, subscription fees, or monthly charges.",
  },
  {
    q: "How long does course review take?",
    a: "Course reviews typically take 2–3 business days. You'll receive a notification when your course is approved or if changes are required. Rejected courses include specific feedback on what needs to be improved.",
  },
  {
    q: "Can I offer discounts and coupons?",
    a: "Yes, you can set a discount price on any paid course. Coupon creation is available in the Pricing tab of the Course Editor. Coupons can be time-limited or limited to a certain number of uses.",
  },
  {
    q: "How do I track student progress?",
    a: "Visit the Students page to see each student's progress percentage and last activity date. The Analytics page provides aggregated completion rates and engagement metrics for all your courses.",
  },
]

export default function HelpPage() {
  const [openIndex, setOpenIndex] = useState<number | null>(null)
  const [subject, setSubject] = useState("general")
  const [message, setMessage] = useState("")

  return (
    <div className="p-4 sm:p-6 max-w-3xl mx-auto space-y-8">
      <div>
        <h2 className="text-lg font-semibold">Help & Support</h2>
        <p className="text-xs text-muted-foreground mt-0.5">Find answers or contact the support team.</p>
      </div>

      {/* FAQ */}
      <div>
        <h3 className="mb-4 text-sm font-medium">Frequently Asked Questions</h3>
        <div className="divide-y divide-border border border-border">
          {FAQS.map((faq, i) => (
            <div key={i}>
              <button
                className="flex w-full items-center justify-between px-4 py-3 text-left text-xs font-medium hover:bg-muted/50 transition-colors"
                onClick={() => setOpenIndex(openIndex === i ? null : i)}
                aria-expanded={openIndex === i}
              >
                <span>{faq.q}</span>
                <HugeiconsIcon
                  icon={Add01Icon}
                  strokeWidth={2}
                  className={cn("h-4 w-4 flex-shrink-0 transition-transform text-muted-foreground", openIndex === i && "rotate-45")}
                />
              </button>
              {openIndex === i && (
                <div className="border-t border-border bg-muted/30 px-4 py-3">
                  <p className="text-xs text-muted-foreground leading-relaxed">{faq.a}</p>
                </div>
              )}
            </div>
          ))}
        </div>
      </div>

      {/* Contact form */}
      <div>
        <h3 className="mb-4 text-sm font-medium">Contact Support</h3>
        <div className="border border-border p-4 space-y-4">
          <p className="text-xs text-muted-foreground">
            Demo — support tickets are not actually submitted. This form is for demonstration purposes only.
          </p>
          <div>
            <Label htmlFor="support-subject" className="text-xs font-medium">Subject</Label>
            <select
              id="support-subject"
              value={subject}
              onChange={(e) => setSubject(e.target.value)}
              className="mt-1 h-8 w-full rounded-none border border-border bg-background px-2 text-xs outline-none focus:ring-1 focus:ring-ring/50"
            >
              <option value="general">General Question</option>
              <option value="technical">Technical Issue</option>
              <option value="billing">Billing & Payments</option>
              <option value="course">Course Approval</option>
              <option value="other">Other</option>
            </select>
          </div>
          <div>
            <Label htmlFor="support-message" className="text-xs font-medium">Message</Label>
            <Textarea
              id="support-message"
              value={message}
              onChange={(e) => setMessage(e.target.value)}
              placeholder="Describe your issue or question in detail..."
              rows={5}
              className="mt-1"
            />
          </div>
          <Button
            size="sm"
            onClick={() => {
              if (!message.trim()) return
              setMessage("")
              toast.success("Support ticket submitted (demo)", { description: "No actual ticket was created." })
            }}
            disabled={!message.trim()}
          >
            Submit Ticket (Demo)
          </Button>
        </div>
      </div>

      {/* Quick links */}
      <div>
        <h3 className="mb-3 text-sm font-medium">Quick Links</h3>
        <div className="grid grid-cols-1 gap-2 sm:grid-cols-2">
          {[
            "Creator Handbook",
            "Video Production Guide",
            "Pricing Strategy Tips",
            "Community Guidelines",
          ].map((link) => (
            <button
              key={link}
              className="flex items-center justify-between border border-border px-3 py-2.5 text-xs hover:bg-muted transition-colors text-left"
              onClick={() => toast.info("Demo", { description: "External links are not available in this demo." })}
            >
              {link}
              <HugeiconsIcon icon={ArrowRight01Icon} strokeWidth={2} className="h-3.5 w-3.5 text-muted-foreground" />
            </button>
          ))}
        </div>
      </div>
    </div>
  )
}
