-- Migration: Backfill slugs for existing institution news, events, and blogs
-- Generates slug from title: lowercase, spaces to dashes, special chars removed, max 80 chars

-- Helper function to generate slug from text
CREATE OR REPLACE FUNCTION generate_slug(input_text TEXT) RETURNS TEXT AS $$
DECLARE
  slug TEXT;
BEGIN
  slug := lower(input_text);
  slug := regexp_replace(slug, '[^a-z0-9\s-]', '', 'g');
  slug := regexp_replace(slug, '\s+', '-', 'g');
  slug := regexp_replace(slug, '-+', '-', 'g');
  slug := regexp_replace(slug, '^-|-$', '', 'g');
  IF length(slug) > 80 THEN
    slug := substring(slug from 1 for 80);
    slug := regexp_replace(slug, '-[^-]*$', '', 'g');
  END IF;
  IF slug = '' OR slug IS NULL THEN
    slug := 'untitled';
  END IF;
  RETURN slug;
END;
$$ LANGUAGE plpgsql;

-- Backfill institution_news slugs
DO $$
DECLARE
  r RECORD;
  base_slug TEXT;
  final_slug TEXT;
  counter INT;
BEGIN
  FOR r IN SELECT id, title FROM institution_news WHERE (slug = '' OR slug IS NULL) AND deleted_at IS NULL
  LOOP
    base_slug := 'inst-' || generate_slug(r.title);
    final_slug := base_slug;
    counter := 1;
    WHILE EXISTS (SELECT 1 FROM institution_news WHERE slug = final_slug AND id != r.id AND deleted_at IS NULL) LOOP
      counter := counter + 1;
      final_slug := base_slug || '-' || counter;
    END LOOP;
    UPDATE institution_news SET slug = final_slug WHERE id = r.id;
  END LOOP;
END $$;

-- Backfill institution_events slugs
DO $$
DECLARE
  r RECORD;
  base_slug TEXT;
  final_slug TEXT;
  counter INT;
BEGIN
  FOR r IN SELECT id, name FROM institution_events WHERE (slug = '' OR slug IS NULL) AND deleted_at IS NULL
  LOOP
    base_slug := 'inst-' || generate_slug(r.name);
    final_slug := base_slug;
    counter := 1;
    WHILE EXISTS (SELECT 1 FROM institution_events WHERE slug = final_slug AND id != r.id AND deleted_at IS NULL) LOOP
      counter := counter + 1;
      final_slug := base_slug || '-' || counter;
    END LOOP;
    UPDATE institution_events SET slug = final_slug WHERE id = r.id;
  END LOOP;
END $$;

-- Backfill institution_blogs slugs
DO $$
DECLARE
  r RECORD;
  base_slug TEXT;
  final_slug TEXT;
  counter INT;
BEGIN
  FOR r IN SELECT id, title FROM institution_blogs WHERE (slug = '' OR slug IS NULL) AND deleted_at IS NULL
  LOOP
    base_slug := 'inst-' || generate_slug(r.title);
    final_slug := base_slug;
    counter := 1;
    WHILE EXISTS (SELECT 1 FROM institution_blogs WHERE slug = final_slug AND id != r.id AND deleted_at IS NULL) LOOP
      counter := counter + 1;
      final_slug := base_slug || '-' || counter;
    END LOOP;
    UPDATE institution_blogs SET slug = final_slug WHERE id = r.id;
  END LOOP;
END $$;

-- Drop helper function
DROP FUNCTION IF EXISTS generate_slug(TEXT);
