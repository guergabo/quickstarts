SELECT
    id, 
    aggregate_type,
    aggregate_id,
    event_type,
    event_payload,
    created_at,
    processed_at,
    status
FROM order_outboxes 
WHERE processed_at IS NULL AND status = 'pending'
ORDER BY created_at ASC
LIMIT $1
FOR UPDATE SKIP LOCKED
