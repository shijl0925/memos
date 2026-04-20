ALTER TABLE
  resource
ADD
  COLUMN public_id TEXT NOT NULL DEFAULT '';

-- TODO(steven): drop this temporary composite index after the public_id-only lookup
-- path has shipped for a full release cycle and older mixed (id, public_id) readers no longer need it.
CREATE UNIQUE INDEX resource_id_public_id_unique_index ON resource (id, public_id);

UPDATE
  resource
SET
  public_id = printf (
    '%s-%s-%s-%s-%s',
    lower(hex(randomblob(4))),
    lower(hex(randomblob(2))),
    lower(hex(randomblob(2))),
    lower(hex(randomblob(2))),
    lower(hex(randomblob(6)))
  );
