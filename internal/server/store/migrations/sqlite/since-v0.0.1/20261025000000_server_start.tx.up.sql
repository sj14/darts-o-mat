-- When a server last started, one row for all replicas: no turn ends earlier than its
-- time after it. Before, each process kept its own start.
CREATE TABLE "server_start" ("id" INTEGER NOT NULL PRIMARY KEY, "started_at" TIMESTAMP NOT NULL);
