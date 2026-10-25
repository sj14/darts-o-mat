-- The failed logins of the last minutes, per username or network, for all replicas.
-- Before, each process counted them in memory.
CREATE TABLE "login_failures" ("key" VARCHAR NOT NULL, "failed_at" TIMESTAMP NOT NULL);

--bun:split

CREATE INDEX "login_failures_key_failed_at" ON "login_failures" ("key", "failed_at");
