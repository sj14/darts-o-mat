-- Every login deletes the sessions that expired, see store.CreateSession.
CREATE INDEX "sessions_expires_at" ON "sessions" ("expires_at");
