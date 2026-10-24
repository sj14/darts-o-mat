-- Invisible accounts look offline to their friends.
ALTER TABLE "accounts" ADD COLUMN "invisible" BOOLEAN NOT NULL DEFAULT false;
