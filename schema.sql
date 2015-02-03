CREATE TABLE "video" (
  "id" serial NOT NULL,
  "youtube_id" character varying(20) NOT NULL,
  "amara_id" character varying(20) NULL,
  "last_checked" timestamp NULL,
  PRIMARY KEY ("id")
);
ALTER TABLE "video" ADD "skip" boolean NOT NULL DEFAULT 'f';

CREATE EXTENSION HSTORE;
ALTER TABLE "public"."video" ADD COLUMN "revisions" hstore NULL;

CREATE INDEX "video_skip_last_checked" ON "video" ("skip", "last_checked" NULLS FIRST);

CREATE TABLE "revision" (
	"id" serial NOT NULL,
	"video_id" int NOT NULL,
	"language" varchar(10) NOT NULL,
	"revision" int NOT NULL,
	"author" varchar(50) NOT NULL,
	"content" hstore NOT NULL,
	"published_at" date NULL,
	PRIMARY KEY ("id")
);

ALTER TABLE "revision" ADD FOREIGN KEY ("video_id") REFERENCES "video" ("id") ON DELETE CASCADE ON UPDATE CASCADE;

CREATE TABLE "author" (
  "id" serial NOT NULL,
  "username" character varying(255) NOT NULL,
  "joined_at" date NOT NULL,
  "first_name" character varying(255) NOT NULL,
  "last_name" character varying(255) NOT NULL,
  "avatar" text NOT NULL,
  PRIMARY KEY ("id")
);
ALTER TABLE "author" ADD CONSTRAINT "author_id" PRIMARY KEY ("id");


