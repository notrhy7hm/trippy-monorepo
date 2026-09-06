# TRIPPY.ai

TRIPPY.ai is a collaborative trip planning platform for groups of friends.

The idea is simple: create a trip, invite people, plan everything together, and track planning, itinerary, and budget details in one workspace.

Current stage: **M3 / budget**.

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

- Docker Compose for PostgreSQL, backend, frontend, and reverse proxy routing

---

## Project Structure

```txt
backend/        Go backend
frontend/       React frontend
docker-compose.yml
```

---

## Local development

Start PostgreSQL:

```sh
docker compose up -d postgres
```

Run backend migrations:

```sh
cd backend
go run github.com/pressly/goose/v3/cmd/goose@latest -dir migrations postgres "postgres://trippy:trippy@localhost:5432/trippy?sslmode=disable" up
```

Run the backend:

```sh
cd backend
cp .env.example .env
go run ./cmd/server
```

Run the frontend:

```sh
cd frontend
npm ci
npm run dev
```

The Vite dev server proxies `/api` to `localhost:8080`.

---

## Container deployment

Create a root `.env` file with a production JWT secret:

```sh
JWT_SECRET=replace-with-at-least-16-random-bytes
ALLOW_ORIGIN=http://YOUR_SERVER_IP
HTTP_PORT=80
```

Build and start the full app:

```sh
docker compose up -d --build
```

The frontend serves the React app on `HTTP_PORT` and proxies `/api/*` plus `/healthz` to the backend service. The backend container runs goose migrations before starting the server.
