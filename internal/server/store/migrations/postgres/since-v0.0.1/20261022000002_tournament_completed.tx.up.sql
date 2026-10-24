-- When a tournament was seen completed (over, every game completed), so the check that a
-- player has one seat at a time leaves it out in SQL. Set when it is next looked at.
ALTER TABLE "tournaments" ADD COLUMN "completed_at" TIMESTAMPTZ;
