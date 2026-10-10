from __future__ import annotations

from datetime import datetime

from sqlalchemy import Date, Float, Integer, String, Text, TIMESTAMP, Boolean
from sqlalchemy.dialects.postgresql import JSONB, UUID
from sqlalchemy.orm import DeclarativeBase, Mapped, mapped_column


class Base(DeclarativeBase):
    pass


class AnalyticsEvent(Base):
    __tablename__ = "analytics_events"

    id: Mapped[UUID] = mapped_column(UUID(as_uuid=True), primary_key=True)
    event_id: Mapped[UUID] = mapped_column(UUID(as_uuid=True), unique=True, nullable=False)
    event_type: Mapped[str] = mapped_column(String(80), nullable=False)
    event_version: Mapped[int] = mapped_column(Integer, nullable=False, default=1)
    occurred_at: Mapped[datetime] = mapped_column(TIMESTAMP(timezone=True), nullable=False)
    received_at: Mapped[datetime] = mapped_column(TIMESTAMP(timezone=True), nullable=False)
    actor_id: Mapped[str | None] = mapped_column(String(128), nullable=True)
    anonymous_id: Mapped[str | None] = mapped_column(String(128), nullable=True)
    session_id: Mapped[str | None] = mapped_column(String(128), nullable=True)
    source_service: Mapped[str] = mapped_column(String(64), nullable=False)
    entity_type: Mapped[str] = mapped_column(String(64), nullable=False)
    entity_id: Mapped[str] = mapped_column(String(128), nullable=False)
    properties: Mapped[dict] = mapped_column(JSONB, nullable=False, default={})
    processing_status: Mapped[str] = mapped_column(String(32), nullable=False, default="pending")
    processed_at: Mapped[datetime | None] = mapped_column(TIMESTAMP(timezone=True), nullable=True)
    created_at: Mapped[datetime] = mapped_column(TIMESTAMP(timezone=True), nullable=False)


class AnalyticsCreatorDaily(Base):
    __tablename__ = "analytics_creator_daily"

    creator_id: Mapped[str] = mapped_column(String(128), primary_key=True)
    reporting_date: Mapped[datetime] = mapped_column(Date, primary_key=True)
    content_views: Mapped[int] = mapped_column(Integer, default=0, nullable=False)
    unique_viewers: Mapped[int] = mapped_column(Integer, default=0, nullable=False)
    followers_gained: Mapped[int] = mapped_column(Integer, default=0, nullable=False)
    course_enrollments: Mapped[int] = mapped_column(Integer, default=0, nullable=False)
    course_completions: Mapped[int] = mapped_column(Integer, default=0, nullable=False)
    watch_time_seconds: Mapped[int] = mapped_column(Integer, default=0, nullable=False)
    revenue_cents: Mapped[int] = mapped_column(Integer, default=0, nullable=False)
    created_at: Mapped[datetime] = mapped_column(TIMESTAMP(timezone=True), nullable=False)
    updated_at: Mapped[datetime] = mapped_column(TIMESTAMP(timezone=True), nullable=False)


class AnalyticsCourseDaily(Base):
    __tablename__ = "analytics_course_daily"

    course_id: Mapped[str] = mapped_column(String(128), primary_key=True)
    reporting_date: Mapped[datetime] = mapped_column(Date, primary_key=True)
    course_views: Mapped[int] = mapped_column(Integer, default=0, nullable=False)
    enrollments: Mapped[int] = mapped_column(Integer, default=0, nullable=False)
    completions: Mapped[int] = mapped_column(Integer, default=0, nullable=False)
    completion_rate: Mapped[float] = mapped_column(Float, default=0.0, nullable=False)
    lesson_activity: Mapped[int] = mapped_column(Integer, default=0, nullable=False)
    watch_time_seconds: Mapped[int] = mapped_column(Integer, default=0, nullable=False)
    created_at: Mapped[datetime] = mapped_column(TIMESTAMP(timezone=True), nullable=False)
    updated_at: Mapped[datetime] = mapped_column(TIMESTAMP(timezone=True), nullable=False)


class AnalyticsPlatformDaily(Base):
    __tablename__ = "analytics_platform_daily"

    reporting_date: Mapped[datetime] = mapped_column(Date, primary_key=True)
    active_users: Mapped[int] = mapped_column(Integer, default=0, nullable=False)
    new_enrollments: Mapped[int] = mapped_column(Integer, default=0, nullable=False)
    course_completions: Mapped[int] = mapped_column(Integer, default=0, nullable=False)
    published_content_count: Mapped[int] = mapped_column(Integer, default=0, nullable=False)
    gross_revenue_cents: Mapped[int] = mapped_column(Integer, default=0, nullable=False)
    refunds_cents: Mapped[int] = mapped_column(Integer, default=0, nullable=False)
    net_revenue_cents: Mapped[int] = mapped_column(Integer, default=0, nullable=False)
    created_at: Mapped[datetime] = mapped_column(TIMESTAMP(timezone=True), nullable=False)
    updated_at: Mapped[datetime] = mapped_column(TIMESTAMP(timezone=True), nullable=False)


class AnalyticsProcessingCheckpoint(Base):
    __tablename__ = "analytics_processing_checkpoints"

    id: Mapped[UUID] = mapped_column(UUID(as_uuid=True), primary_key=True)
    checkpoint_name: Mapped[str] = mapped_column(String(128), unique=True, nullable=False)
    last_event_id: Mapped[UUID | None] = mapped_column(UUID(as_uuid=True), nullable=True)
    last_processed_at: Mapped[datetime | None] = mapped_column(TIMESTAMP(timezone=True), nullable=True)
    created_at: Mapped[datetime] = mapped_column(TIMESTAMP(timezone=True), nullable=False)
    updated_at: Mapped[datetime] = mapped_column(TIMESTAMP(timezone=True), nullable=False)
