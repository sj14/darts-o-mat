-- Whether registering needs the captcha, if the server has its keys: the admin can turn it off.
ALTER TABLE "site_settings" ADD COLUMN "captcha" BOOLEAN NOT NULL DEFAULT true;
