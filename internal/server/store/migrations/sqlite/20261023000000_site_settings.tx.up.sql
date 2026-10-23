-- The admin's settings of the server, one row: whether anyone may register.
CREATE TABLE "site_settings" ("id" INTEGER NOT NULL PRIMARY KEY, "registration" BOOLEAN NOT NULL DEFAULT true);

--bun:split

INSERT INTO "site_settings" ("id", "registration") VALUES (1, true);
