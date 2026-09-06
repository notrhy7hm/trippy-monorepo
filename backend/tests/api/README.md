# backend/tests/api

DB-backed HTTP integration tests for the M1-M3 API surface: auth, friends,
trips, planning, itinerary, and budget.

## Safety

`TestMain` refuses to run unless **both** of these are true:

1. `TEST_DATABASE_URL` is set.
2. The database name in that URL contains the substring `test`
   (case-insensitive).

Without `TEST_DATABASE_URL`, all tests in this package are skipped
(exit 0) so the rest of `go test ./...` is unaffected. If the URL is
present but the database name doesn't look like a test DB, the suite
**fatals** rather than silently truncating something it shouldn't.

`cleanDB` runs at the start of every test and issues a single
`TRUNCATE … RESTART IDENTITY CASCADE` against every user-data table.
Do not point `TEST_DATABASE_URL` at the dev or production database.

## Creating the test database

The dev compose stack already runs Postgres. Create a separate
database next to the dev one:

```powershell
# Using your dev DB user. Replace the password if your local user differs.
$env:PGPASSWORD = "trippy"
psql -h localhost -U trippy -d postgres -c "CREATE DATABASE trippy_test;"

# Optional: a dedicated test user
psql -h localhost -U trippy -d postgres -c "CREATE USER trippy_test WITH PASSWORD 'trippy_test';"
psql -h localhost -U trippy -d postgres -c "GRANT ALL PRIVILEGES ON DATABASE trippy_test TO trippy_test;"
```

## Running migrations

The test setup does **not** auto-migrate. Run goose against the test DB
before the first run and whenever a new migration lands. The schema smoke check
expects the M3 budget tables from migration `0006_budget.sql`.

```powershell
$env:TEST_DATABASE_URL = "postgres://trippy:trippy@localhost:5432/trippy_test?sslmode=disable"

cd backend
go run github.com/pressly/goose/v3/cmd/goose@latest -dir migrations postgres "$env:TEST_DATABASE_URL" up
```

## Running the tests

```powershell
$env:TEST_DATABASE_URL = "postgres://trippy:trippy@localhost:5432/trippy_test?sslmode=disable"

cd backend
go test ./tests/api -v
```

If `TEST_DATABASE_URL` is unset, the package skips silently — the rest
of `go test ./...` still runs.

## Adding new tests

* Always start the test body with `cleanDB(t)`.
* Use `registerUser` / `createTrip` / etc. — keep new helpers tight
  and focused.
* Use `mustDo` for straight-line happy paths and `doRequest` /
  `rawDo` (the goroutine-safe variant) when you need to inspect the
  status or body yourself.
* Never call `t.Parallel()` in this package: every test reuses the
  same DB.
