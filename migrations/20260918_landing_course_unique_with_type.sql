-- Landing-course institution links: make uniqueness type-aware.
-- colleges and institution_users are different tables with independent numeric
-- IDs, so having both a type='college' row and a type='institution' row that
-- share the same institution_id is legitimate (even when the IDs collide).
-- The previous UNIQUE(field_id, institution_id) blocked that second link.
-- Legacy note: the dropped constraint guarantees no duplicate
-- (field_id, institution_id) pairs exist, so adding the wider unique
-- (which is a superset) cannot fail on existing data.

ALTER TABLE landing_course_institutions
DROP CONSTRAINT IF EXISTS landing_course_institutions_field_id_institution_id_key;

ALTER TABLE landing_course_institutions
DROP CONSTRAINT IF EXISTS uc_landing_course_institutions;

ALTER TABLE landing_course_institutions
ADD CONSTRAINT uc_landing_course_institutions UNIQUE (field_id, institution_id, institution_type);
