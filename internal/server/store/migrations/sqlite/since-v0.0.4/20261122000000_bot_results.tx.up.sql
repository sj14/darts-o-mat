-- A result of a game with a bot counts for the darts, not as a game played or won.
ALTER TABLE "game_stats" ADD COLUMN "bots" BOOLEAN NOT NULL DEFAULT false;
