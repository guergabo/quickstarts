SELECT
    id, 
    amount, 
    created_at, 
    updated_at, 
    status
FROM orders
WHERE id = $1;
