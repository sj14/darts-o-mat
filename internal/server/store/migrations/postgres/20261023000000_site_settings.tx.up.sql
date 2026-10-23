-- The admin's settings of the server, one row: whether anyone may register.
CREATE TABLE "site_settings" ("id" BIGINT NOT NULL, "registration" BOOLEAN NOT NULL DEFAULT true, PRIMARY KEY ("id"));

--bun:split

INSERT INTO "site_settings" ("id", "registration") VALUES (1, true);
