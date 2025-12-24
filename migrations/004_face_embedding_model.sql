ALTER TABLE face_embeddings
  ADD COLUMN model VARCHAR(50) NULL AFTER embedding;

UPDATE face_embeddings
SET model = 'arcface'
WHERE model IS NULL;

