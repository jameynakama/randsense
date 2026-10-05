-- built sentences were kept from the builder; generated ones came from random.
ALTER TABLE sentences ADD COLUMN origin TEXT NOT NULL DEFAULT 'generated'
    CHECK (origin IN ('generated', 'built'));
