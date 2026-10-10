from __future__ import annotations

from fastapi import FastAPI

from app.api import ai, health
from app.db.session import initialize_database

app = FastAPI(title="Edvance AI Service", version="0.1.0")


@app.on_event("startup")
async def startup_event() -> None:
    await initialize_database()


app.include_router(health.router)
app.include_router(ai.router, prefix="/api/v1")


if __name__ == "__main__":
    import uvicorn

    uvicorn.run("app.main:app", host="0.0.0.0", port=8095, reload=True)
