WITH new_order AS (
    INSERT INTO orders (
        amount, 
        currency, 
        customer_id, 
        description,
        created_at
    )
    VALUES (
        $1, 
        $2, 
        $3, 
        $4, 
        $5
    )
    RETURNING *
),
order_event AS (
    INSERT INTO order_outboxes (
        aggregate_type, 
        aggregate_id, 
        event_type,
        event_payload,
        created_at
    )
    SELECT 
        'orders', -- TODO: align with NATs.
        id,
        'ORDER_CREATED',
        jsonb_build_object( 
            'amount', amount,
            'currency', currency,
            'customer', customer_id,
            'description', description
        ),
        created_at
    FROM new_order
    RETURNING *
)
SELECT
    o.*,
    e.*
FROM new_order o 
JOIN order_event e ON e.aggregate_id = o.id; 
