-- Bots are guests the server throws for; their strength belongs to the seat.
ALTER TABLE "players" ADD COLUMN "bot" BOOLEAN NOT NULL DEFAULT false;

--bun:split

ALTER TABLE "game_players" ADD COLUMN "bot_spread_um" INTEGER NOT NULL DEFAULT 0;
