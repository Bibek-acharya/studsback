-- Migration: Backfill slugs for existing institution news, events, and blogs
-- Generates slug from title: inst-{id}-{title-with-dashes}

-- Backfill institution_news slugs
UPDATE institution_news
SET slug = 'inst-' || id || '-' || LOWER(REPLACE(REPLACE(REPLACE(title, ' ', '-'), '''', ''), '&', ''))
WHERE (slug IS NULL OR slug = '') AND deleted_at IS NULL;

-- Backfill institution_events slugs
UPDATE institution_events
SET slug = 'inst-' || id || '-' || LOWER(REPLACE(REPLACE(REPLACE(name, ' ', '-'), '''', ''), '&', ''))
WHERE (slug IS NULL OR slug = '') AND deleted_at IS NULL;

-- Backfill institution_blogs slugs
UPDATE institution_blogs
SET slug = 'inst-' || id || '-' || LOWER(REPLACE(REPLACE(REPLACE(title, ' ', '-'), '''', ''), '&', ''))
WHERE (slug IS NULL OR slug = '') AND deleted_at IS NULL;
