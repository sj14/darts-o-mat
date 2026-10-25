-- When a server last started, one row for all replicas: no turn ends earlier than its
-- time after it. Before, each process kept its own start.
CREATE TABLE "server_start" ("id" BIGINT NOT NULL, "started_at" TIMESTAMPTZ NOT NULL, PRIMARY KEY ("id"));
