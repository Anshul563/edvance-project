from __future__ import annotations

import re

from app.config import get_settings

settings = get_settings()


class ContextService:
    @staticmethod
    def normalize_text(text: str | None) -> str:
        if not text:
            return ""
        return re.sub(r"\s+", " ", text).strip()

    @staticmethod
    def split_chunks(text: str, *, chunk_size: int = 1500, overlap: int = 200) -> list[str]:
        cleaned = ContextService.normalize_text(text)
        if not cleaned:
            return []
        chunks: list[str] = []
        start = 0
        while start < len(cleaned):
            end = min(len(cleaned), start + chunk_size)
            chunk = cleaned[start:end]
            chunks.append(chunk)
            if end >= len(cleaned):
                break
            start = max(start + chunk_size - overlap, end - overlap)
        return chunks

    @staticmethod
    def rank_chunks(chunks: list[str], query: str) -> list[tuple[str, float]]:
        normalized_query = ContextService.normalize_text(query).lower()
        if not normalized_query:
            return [(chunk, 1.0) for chunk in chunks]
        ranked: list[tuple[str, float]] = []
        for chunk in chunks:
            text = chunk.lower()
            score = 0.0
            query_terms = re.findall(r"[a-z0-9]+", normalized_query)
            for term in query_terms:
                score += text.count(term)
            if any(term in text for term in query_terms):
                score += 0.3 * len(query_terms)
            ranked.append((chunk, score))
        ranked.sort(key=lambda item: item[1], reverse=True)
        return ranked

    @staticmethod
    def select_context(chunks: list[str], query: str, *, max_chars: int | None = None) -> str:
        maximum = max_chars or int(settings["ai_max_context_chars"])
        ranked = ContextService.rank_chunks(chunks, query)
        selected: list[str] = []
        total = 0
        for chunk, _ in ranked:
            if total + len(chunk) > maximum:
                continue
            selected.append(chunk)
            total += len(chunk)
        if not selected:
            return "\n\n".join(chunks[:1]) if chunks else ""
        return "\n\n".join(selected)
