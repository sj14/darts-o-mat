-- The first account registered is the admin; of an existing database the oldest one.
ALTER TABLE "accounts" ADD COLUMN "admin" BOOLEAN NOT NULL DEFAULT false;

--bun:split

UPDATE "accounts" SET "admin" = true WHERE "id" = (SELECT min("id") FROM "accounts");

--bun:split

CREATE TABLE "pages" ("name" VARCHAR NOT NULL, "text" VARCHAR NOT NULL, PRIMARY KEY ("name"));
