-- When the account was last online, for its friends. Before, each process kept it in
-- memory.
ALTER TABLE "accounts" ADD COLUMN "last_seen_at" TIMESTAMPTZ;
