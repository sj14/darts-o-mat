-- X01 turns counted by 60+, 100+, 140+, 170+ and 180. Games completed before keep 0 in
-- the new columns and their 170 to 179 in turns140: their darts aren't replayed.
ALTER TABLE "game_stats" ADD COLUMN "turns60" BIGINT;
--bun:split
ALTER TABLE "game_stats" ADD COLUMN "turns170" BIGINT;
