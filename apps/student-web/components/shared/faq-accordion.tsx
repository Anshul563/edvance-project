"use client"

import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from "@/components/ui/accordion"

export function FaqAccordion({
  items,
}: {
  items: { question: string; answer: string }[]
}) {
  return (
    <Accordion
      defaultValue={[items[0]?.question ?? ""]}
      className="border border-border"
    >
      {items.map((item) => (
        <AccordionItem
          key={item.question}
          value={item.question}
          className="not-last:border-b"
        >
          <AccordionTrigger className="px-4 py-3.5 text-sm hover:no-underline">
            {item.question}
          </AccordionTrigger>
          <AccordionContent className="px-4 text-sm leading-relaxed text-muted-foreground">
            {item.answer}
          </AccordionContent>
        </AccordionItem>
      ))}
    </Accordion>
  )
}
