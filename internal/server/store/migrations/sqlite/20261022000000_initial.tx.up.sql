-- The schema the tables had before the migrations; IF NOT EXISTS lets databases made then take it as their start.

CREATE TABLE IF NOT EXISTS "accounts" ("id" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT, "username" VARCHAR NOT NULL, "email" VARCHAR NOT NULL DEFAULT '', "password_hash" VARCHAR NOT NULL, "player_id" INTEGER NOT NULL, "language" VARCHAR NOT NULL DEFAULT '', "created_at" TIMESTAMP NOT NULL DEFAULT current_timestamp, UNIQUE ("username"), UNIQUE ("player_id"));

--bun:split

CREATE TABLE IF NOT EXISTS "sessions" ("token_hash" VARCHAR NOT NULL, "account_id" INTEGER NOT NULL, "expires_at" TIMESTAMP NOT NULL, PRIMARY KEY ("token_hash"));

--bun:split

CREATE TABLE IF NOT EXISTS "friendships" ("requester_id" INTEGER NOT NULL, "addressee_id" INTEGER NOT NULL, "status" VARCHAR NOT NULL, "created_at" TIMESTAMP NOT NULL DEFAULT current_timestamp, PRIMARY KEY ("requester_id", "addressee_id"));

--bun:split

CREATE TABLE IF NOT EXISTS "games" ("id" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT, "mode" VARCHAR NOT NULL, "legs" INTEGER NOT NULL DEFAULT 1, "sets" INTEGER NOT NULL DEFAULT 1, "bull_off" BOOLEAN NOT NULL DEFAULT false, "turn_seconds" INTEGER NOT NULL DEFAULT 0, "settings_id" INTEGER NOT NULL DEFAULT 0, "seats" INTEGER NOT NULL DEFAULT 0, "board_only" BOOLEAN NOT NULL DEFAULT false, "bull_off_closer" INTEGER, "created_at" TIMESTAMP NOT NULL DEFAULT current_timestamp, "started_at" TIMESTAMP, "completed_at" TIMESTAMP, "aborted_at" TIMESTAMP, "tournament_id" INTEGER, "tournament_match" INTEGER NOT NULL DEFAULT 0);

--bun:split

CREATE TABLE IF NOT EXISTS "game_settings_x01" ("id" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT, "start_score" INTEGER, "out_mode" INTEGER);

--bun:split

CREATE TABLE IF NOT EXISTS "game_settings_around_the_clock" ("id" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT, "bull" BOOLEAN, "multiplier_skips" BOOLEAN);

--bun:split

CREATE TABLE IF NOT EXISTS "game_players" ("game_id" INTEGER NOT NULL, "player_id" INTEGER NOT NULL, "place" INTEGER NOT NULL, "status" VARCHAR NOT NULL, "board_id" INTEGER, "created_at" TIMESTAMP NOT NULL DEFAULT current_timestamp, PRIMARY KEY ("game_id", "player_id"));

--bun:split

CREATE TABLE IF NOT EXISTS "players" ("id" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT, "name" VARCHAR NOT NULL, "guest_of" INTEGER, "color" VARCHAR, "deleted_at" TIMESTAMP);

--bun:split

CREATE TABLE IF NOT EXISTS "boards" ("id" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT, "account_id" INTEGER NOT NULL, "name" VARCHAR NOT NULL, "created_at" TIMESTAMP NOT NULL DEFAULT current_timestamp, "last_used_at" TIMESTAMP);

--bun:split

CREATE TABLE IF NOT EXISTS "board_tokens" ("board_id" INTEGER NOT NULL, "token_hash" VARCHAR NOT NULL, "created_at" TIMESTAMP NOT NULL DEFAULT current_timestamp, PRIMARY KEY ("board_id"), UNIQUE ("token_hash"));

--bun:split

CREATE TABLE IF NOT EXISTS "darts" ("id" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT, "game_id" INTEGER NOT NULL, "number" INTEGER, "segment" INTEGER NOT NULL, "takeout" BOOLEAN NOT NULL DEFAULT false, "player_id" INTEGER NOT NULL DEFAULT 0, "leave" BOOLEAN NOT NULL DEFAULT false, "skip" BOOLEAN NOT NULL DEFAULT false, "x" DOUBLE PRECISION, "y" DOUBLE PRECISION, "created_at" TIMESTAMP NOT NULL DEFAULT current_timestamp);

--bun:split

CREATE TABLE IF NOT EXISTS "game_stats" ("game_id" INTEGER NOT NULL, "player_id" INTEGER NOT NULL, "mode" VARCHAR NOT NULL, "won" BOOLEAN NOT NULL, "legs" INTEGER, "sets" INTEGER, "best_leg" INTEGER, "game_time" INTEGER, "leg_time" INTEGER, "timed_legs" INTEGER, "darts" INTEGER, "points" INTEGER, "highest_turn" INTEGER, "turns100" INTEGER, "turns140" INTEGER, "turns180" INTEGER, "checkout" INTEGER, "hits" INTEGER, PRIMARY KEY ("game_id", "player_id"));

--bun:split

CREATE TABLE IF NOT EXISTS "tournament_stats" ("tournament_id" INTEGER NOT NULL, "player_id" INTEGER NOT NULL, "mode" VARCHAR NOT NULL, "won" BOOLEAN NOT NULL, PRIMARY KEY ("tournament_id", "player_id"));

--bun:split

CREATE TABLE IF NOT EXISTS "quits" ("player_id" INTEGER NOT NULL, "tournament" BOOLEAN NOT NULL, "target_id" INTEGER NOT NULL, "mode" VARCHAR NOT NULL, PRIMARY KEY ("player_id", "tournament", "target_id"));

--bun:split

CREATE TABLE IF NOT EXISTS "ratings" ("game_id" INTEGER NOT NULL, "rater_id" INTEGER NOT NULL, "rated_id" INTEGER NOT NULL, "up" BOOLEAN NOT NULL, PRIMARY KEY ("game_id", "rater_id", "rated_id"));

--bun:split

CREATE TABLE IF NOT EXISTS "tournaments" ("id" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT, "format" VARCHAR NOT NULL, "mode" VARCHAR NOT NULL, "legs" INTEGER NOT NULL DEFAULT 1, "sets" INTEGER NOT NULL DEFAULT 1, "bull_off" BOOLEAN NOT NULL DEFAULT false, "turn_seconds" INTEGER NOT NULL DEFAULT 0, "settings_id" INTEGER NOT NULL DEFAULT 0, "seats" INTEGER NOT NULL DEFAULT 0, "board_only" BOOLEAN NOT NULL DEFAULT false, "created_at" TIMESTAMP NOT NULL DEFAULT current_timestamp, "aborted_at" TIMESTAMP);

--bun:split

CREATE TABLE IF NOT EXISTS "tournament_players" ("tournament_id" INTEGER NOT NULL, "player_id" INTEGER NOT NULL, "place" INTEGER NOT NULL, "status" VARCHAR NOT NULL, "board_id" INTEGER, "created_at" TIMESTAMP NOT NULL DEFAULT current_timestamp, PRIMARY KEY ("tournament_id", "player_id"));

--bun:split

CREATE UNIQUE INDEX IF NOT EXISTS "accounts_email_key" ON "accounts" ("email") WHERE (email <> '');
