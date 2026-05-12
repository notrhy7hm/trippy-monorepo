# TRIPPY.ai

TRIPPY.ai is a collaborative trip planning platform for groups of friends.

The idea is simple: create a trip, invite people, plan everything together, and later use a trip-scoped AI agent to help with planning, reminders, budgets, summaries, and other trip-related tasks.

Current stage: **M0 / early foundation**.

---

## Tech Stack

**Backend**
- Go
- PostgreSQL
- chi router
- sqlx + pgx
- JWT auth
- goose migrations

**Frontend**
- React
- Vite
- TypeScript
- Tailwind CSS
- Inter font

**Infrastructure**
- Docker Compose for local PostgreSQL

---

## Project Structure

```txt
backend/        Go backend
frontend/       React frontend
docker-compose.yml