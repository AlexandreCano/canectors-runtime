INSERT INTO dest_customers (external_id, email, full_name, segment, source_updated_at, synced_at)
VALUES ($1, $2, $3, $4, $5, NOW())
ON CONFLICT (external_id) DO UPDATE SET
    email = EXCLUDED.email,
    full_name = EXCLUDED.full_name,
    segment = EXCLUDED.segment,
    source_updated_at = EXCLUDED.source_updated_at,
    synced_at = NOW()
