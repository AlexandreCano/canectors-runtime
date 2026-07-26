insert into products (product_id, sku, price, updated_at)
values ($1, $2, $3, $4)
on conflict (product_id) do update set
  sku = excluded.sku,
  price = excluded.price,
  updated_at = excluded.updated_at
