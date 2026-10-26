-- The settings of each account's last new game or tournament per mode, which the start
-- form shows again.
CREATE TABLE "last_settings" ("account_id" BIGINT NOT NULL, "mode" VARCHAR NOT NULL, "settings" VARCHAR NOT NULL, PRIMARY KEY ("account_id", "mode"));
