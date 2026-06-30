# Quill

AI-powered writing IDE for creative writers with persistent memory.

> *"Quill remembers your story better than you do."*

## Quick Start

```bash
# 1. Copy environment variables
cp .env.example .env

# 2. Edit .env with your Qwen API key
# QWEN_API_KEY=sk-your-key-here

# 3. Start everything
docker compose up -d

# 4. Open the app
open http://localhost:3000
```

## Architecture

- **Backend**: Go (Fiber v2.52.x) + PostgreSQL 16 (pgvector + Apache AGE)
- **Frontend**: React + Vite + TipTap + React Flow
- **AI**: Qwen Cloud API (qwen-max, qwen-turbo, text-embedding-v3)

## Development

```bash
# Backend only
cd backend && go run cmd/server/main.go

# Frontend only
cd frontend && npm run dev

# Database only
docker compose up postgres
```

## Tech Stack

| Component | Technology |
|-----------|-----------|
| Backend | Go 1.22+, Fiber v2.52.x |
| Database | PostgreSQL 16 |
| Vector Search | pgvector |
| Graph Database | Apache AGE |
| Frontend | React 18, Vite, TypeScript |
| Editor | TipTap |
| Graph Viz | React Flow |
| State | Zustand |
| AI | Qwen Cloud API |

## License

MIT
