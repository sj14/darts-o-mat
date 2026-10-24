-- The schema the tables had before the migrations; IF NOT EXISTS lets databases made then take it as their start.

CREATE TABLE IF NOT EXISTS "accounts" ("id" BIGSERIAL NOT NULL, "username" VARCHAR NOT NULL, "email" VARCHAR NOT NULL DEFAULT '', "password_hash" VARCHAR NOT NULL, "player_id" BIGINT NOT NULL, "language" VARCHAR NOT NULL DEFAULT '', "created_at" TIMESTAMPTZ NOT NULL DEFAULT current_timestamp, PRIMARY KEY ("id"), UNIQUE ("username"), UNIQUE ("player_id"));

--bun:split

CREATE TABLE IF NOT EXISTS "sessions" ("token_hash" VARCHAR NOT NULL, "account_id" BIGINT NOT NULL, "expires_at" TIMESTAMPTZ NOT NULL, PRIMARY KEY ("token_hash"));

--bun:split

CREATE TABLE IF NOT EXISTS "friendships" ("requester_id" BIGINT NOT NULL, "addressee_id" BIGINT NOT NULL, "status" VARCHAR NOT NULL, "created_at" TIMESTAMPTZ NOT NULL DEFAULT current_timestamp, PRIMARY KEY ("requester_id", "addressee_id"));

--bun:split

CREATE TABLE IF NOT EXISTS "games" ("id" BIGSERIAL NOT NULL, "mode" VARCHAR NOT NULL, "legs" BIGINT NOT NULL DEFAULT 1, "sets" BIGINT NOT NULL DEFAULT 1, "bull_off" BOOLEAN NOT NULL DEFAULT false, "turn_seconds" BIGINT NOT NULL DEFAULT 0, "settings_id" BIGINT NOT NULL DEFAULT 0, "seats" BIGINT NOT NULL DEFAULT 0, "board_only" BOOLEAN NOT NULL DEFAULT false, "bull_off_closer" BIGINT, "created_at" TIMESTAMPTZ NOT NULL DEFAULT current_timestamp, "started_at" TIMESTAMPTZ, "completed_at" TIMESTAMPTZ, "aborted_at" TIMESTAMPTZ, "tournament_id" BIGINT, "tournament_match" BIGINT NOT NULL DEFAULT 0, PRIMARY KEY ("id"));

--bun:split

CREATE TABLE IF NOT EXISTS "game_settings_x01" ("id" BIGSERIAL NOT NULL, "start_score" BIGINT, "out_mode" SMALLINT, PRIMARY KEY ("id"));

--bun:split

CREATE TABLE IF NOT EXISTS "game_settings_around_the_clock" ("id" BIGSERIAL NOT NULL, "bull" BOOLEAN, "multiplier_skips" BOOLEAN, PRIMARY KEY ("id"));

--bun:split

CREATE TABLE IF NOT EXISTS "game_players" ("game_id" BIGINT NOT NULL, "player_id" BIGINT NOT NULL, "place" BIGINT NOT NULL, "status" VARCHAR NOT NULL, "board_id" BIGINT, "created_at" TIMESTAMPTZ NOT NULL DEFAULT current_timestamp, PRIMARY KEY ("game_id", "player_id"));

--bun:split

CREATE TABLE IF NOT EXISTS "players" ("id" BIGSERIAL NOT NULL, "name" VARCHAR NOT NULL, "guest_of" BIGINT, "color" VARCHAR, "deleted_at" TIMESTAMPTZ, PRIMARY KEY ("id"));

--bun:split

CREATE TABLE IF NOT EXISTS "boards" ("id" BIGSERIAL NOT NULL, "account_id" BIGINT NOT NULL, "name" VARCHAR NOT NULL, "created_at" TIMESTAMPTZ NOT NULL DEFAULT current_timestamp, "last_used_at" TIMESTAMPTZ, PRIMARY KEY ("id"));

--bun:split

CREATE TABLE IF NOT EXISTS "board_tokens" ("board_id" BIGINT NOT NULL, "token_hash" VARCHAR NOT NULL, "created_at" TIMESTAMPTZ NOT NULL DEFAULT current_timestamp, PRIMARY KEY ("board_id"), UNIQUE ("token_hash"));

--bun:split

CREATE TABLE IF NOT EXISTS "darts" ("id" BIGSERIAL NOT NULL, "game_id" BIGINT NOT NULL, "number" SMALLINT, "segment" SMALLINT NOT NULL, "takeout" BOOLEAN NOT NULL DEFAULT false, "player_id" BIGINT NOT NULL DEFAULT 0, "leave" BOOLEAN NOT NULL DEFAULT false, "skip" BOOLEAN NOT NULL DEFAULT false, "x" DOUBLE PRECISION, "y" DOUBLE PRECISION, "created_at" TIMESTAMPTZ NOT NULL DEFAULT current_timestamp, PRIMARY KEY ("id"));

--bun:split

CREATE TABLE IF NOT EXISTS "game_stats" ("game_id" BIGINT NOT NULL, "player_id" BIGINT NOT NULL, "mode" VARCHAR NOT NULL, "won" BOOLEAN NOT NULL, "legs" BIGINT, "sets" BIGINT, "best_leg" BIGINT, "game_time" BIGINT, "leg_time" BIGINT, "timed_legs" BIGINT, "darts" BIGINT, "points" BIGINT, "highest_turn" BIGINT, "turns100" BIGINT, "turns140" BIGINT, "turns180" BIGINT, "checkout" BIGINT, "hits" BIGINT, PRIMARY KEY ("game_id", "player_id"));

--bun:split

CREATE TABLE IF NOT EXISTS "tournament_stats" ("tournament_id" BIGINT NOT NULL, "player_id" BIGINT NOT NULL, "mode" VARCHAR NOT NULL, "won" BOOLEAN NOT NULL, PRIMARY KEY ("tournament_id", "player_id"));

--bun:split

CREATE TABLE IF NOT EXISTS "quits" ("player_id" BIGINT NOT NULL, "tournament" BOOLEAN NOT NULL, "target_id" BIGINT NOT NULL, "mode" VARCHAR NOT NULL, PRIMARY KEY ("player_id", "tournament", "target_id"));

--bun:split

CREATE TABLE IF NOT EXISTS "ratings" ("game_id" BIGINT NOT NULL, "rater_id" BIGINT NOT NULL, "rated_id" BIGINT NOT NULL, "up" BOOLEAN NOT NULL, PRIMARY KEY ("game_id", "rater_id", "rated_id"));

--bun:split

CREATE TABLE IF NOT EXISTS "tournaments" ("id" BIGSERIAL NOT NULL, "format" VARCHAR NOT NULL, "mode" VARCHAR NOT NULL, "legs" BIGINT NOT NULL DEFAULT 1, "sets" BIGINT NOT NULL DEFAULT 1, "bull_off" BOOLEAN NOT NULL DEFAULT false, "turn_seconds" BIGINT NOT NULL DEFAULT 0, "settings_id" BIGINT NOT NULL DEFAULT 0, "seats" BIGINT NOT NULL DEFAULT 0, "board_only" BOOLEAN NOT NULL DEFAULT false, "created_at" TIMESTAMPTZ NOT NULL DEFAULT current_timestamp, "aborted_at" TIMESTAMPTZ, PRIMARY KEY ("id"));

--bun:split

CREATE TABLE IF NOT EXISTS "tournament_players" ("tournament_id" BIGINT NOT NULL, "player_id" BIGINT NOT NULL, "place" BIGINT NOT NULL, "status" VARCHAR NOT NULL, "board_id" BIGINT, "created_at" TIMESTAMPTZ NOT NULL DEFAULT current_timestamp, PRIMARY KEY ("tournament_id", "player_id"));

--bun:split

CREATE UNIQUE INDEX IF NOT EXISTS "accounts_email_key" ON "accounts" ("email") WHERE (email <> '');
