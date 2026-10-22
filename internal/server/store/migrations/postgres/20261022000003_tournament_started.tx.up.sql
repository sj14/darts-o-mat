-- When a tournament left its lobby: a player starts it, or the last seat of an open one is
-- taken. Before, it started by itself once no invitation was pending and no seat was free.
ALTER TABLE "tournaments" ADD COLUMN "started_at" TIMESTAMPTZ;

--bun:split

UPDATE "tournaments" SET "started_at" = "created_at"
WHERE NOT EXISTS (SELECT 1 FROM "tournament_players" AS p WHERE p."tournament_id" = "tournaments"."id" AND p."status" = 'pending')
AND ("seats" = 0 OR "seats" <= (SELECT count(*) FROM "tournament_players" AS p WHERE p."tournament_id" = "tournaments"."id"));
