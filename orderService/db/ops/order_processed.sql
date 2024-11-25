UPDATE order_outboxes 
SET 
    processed_at = $1,
    status = 'succeeded'
WHERE id = $2
RETURNING *
