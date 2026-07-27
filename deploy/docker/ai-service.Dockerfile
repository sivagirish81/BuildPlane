FROM python:3.13-slim AS runtime

ENV PYTHONDONTWRITEBYTECODE=1
ENV PYTHONUNBUFFERED=1
ENV PORT=8090

WORKDIR /app

COPY ai-service/pyproject.toml ./
COPY ai-service/app ./app
COPY ai-service/prompts ./prompts

RUN pip install --no-cache-dir ".[openai]"

USER 65532:65532

EXPOSE 8090

CMD ["python", "-m", "uvicorn", "app.main:app", "--host", "0.0.0.0", "--port", "8090"]

