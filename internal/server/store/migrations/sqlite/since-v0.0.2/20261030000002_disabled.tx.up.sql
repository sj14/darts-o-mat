-- An admin can disable an account, so it can't log in.
ALTER TABLE "accounts" ADD COLUMN "disabled" BOOLEAN NOT NULL DEFAULT false;
