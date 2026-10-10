FROM python:3.12-slim
WORKDIR /app
RUN apt-get update && apt-get install -y --no-install-recommends curl && rm -rf /var/lib/apt/lists/*
COPY pyproject.toml ./
COPY . .
RUN pip install --no-cache-dir --upgrade pip && pip install --no-cache-dir -e . && useradd --create-home --uid 10001 app && chown -R app:app /app
USER 10001:10001
EXPOSE 8093
CMD ["sh", "-c", "uvicorn app.main:app --host 0.0.0.0 --port ${RECOMMENDATION_SERVICE_PORT:-8093}"]
