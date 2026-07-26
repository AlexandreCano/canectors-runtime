SELECT segment, region, active
FROM customer_reference
WHERE external_id = $1
LIMIT 1
