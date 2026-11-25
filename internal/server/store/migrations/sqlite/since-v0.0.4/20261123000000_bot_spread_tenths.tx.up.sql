-- A bot's spread is stored in tenths of a mm and is at least 4 mm.
UPDATE "game_players" SET "bot_spread" = MAX("bot_spread", 4) * 10 WHERE "bot_spread" > 0;
