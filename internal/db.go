package internal

import (
    "context"
    "time"

    "github.com/jackc/pgx/v5/pgxpool"
)

type DB struct {
    Pool *pgxpool.Pool
}

func NewDB(ctx context.Context, dsn string) (*DB, error) {
    cfg, err := pgxpool.ParseConfig(dsn)
    if err != nil {
        return nil, err
    }
    // optional: connection settings
    cfg.MaxConns = 10
    pool, err := pgxpool.NewWithConfig(ctx, cfg)
    if err != nil {
        return nil, err
    }
    return &DB{Pool: pool}, nil
}

func (db *DB) Close() {
    db.Pool.Close()
}

// UpsertOrder writes order + delivery + payment + items in a transaction
func (db *DB) UpsertOrder(ctx context.Context, o *Order) error {
    tx, err := db.Pool.Begin(ctx)
    if err != nil {
        return err
    }
    defer func() {
        _ = tx.Rollback(ctx)
    }()

    // upsert orders
    _, err = tx.Exec(ctx, `
    INSERT INTO orders (order_uid, track_number, entry, locale, internal_signature, customer_id, delivery_service, shardkey, sm_id, date_created, oof_shard, created_at, updated_at)
    VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11, now(), now())
    ON CONFLICT (order_uid) DO UPDATE SET
      track_number = EXCLUDED.track_number,
      entry = EXCLUDED.entry,
      locale = EXCLUDED.locale,
      internal_signature = EXCLUDED.internal_signature,
      customer_id = EXCLUDED.customer_id,
      delivery_service = EXCLUDED.delivery_service,
      shardkey = EXCLUDED.shardkey,
      sm_id = EXCLUDED.sm_id,
      date_created = EXCLUDED.date_created,
      oof_shard = EXCLUDED.oof_shard,
      updated_at = now()
    `,
        o.OrderUID, o.TrackNumber, o.Entry, o.Locale, o.InternalSignature, o.CustomerID, o.DeliveryService, o.Shardkey, o.SmID, o.DateCreated, o.OofShard,
    )
    if err != nil {
        return err
    }

    // upsert delivery
    _, err = tx.Exec(ctx, `
    INSERT INTO delivery (order_uid, name, phone, zip, city, address, region, email)
    VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
    ON CONFLICT (order_uid) DO UPDATE SET
      name = EXCLUDED.name,
      phone = EXCLUDED.phone,
      zip = EXCLUDED.zip,
      city = EXCLUDED.city,
      address = EXCLUDED.address,
      region = EXCLUDED.region,
      email = EXCLUDED.email
    `, o.OrderUID, o.Delivery.Name, o.Delivery.Phone, o.Delivery.Zip, o.Delivery.City, o.Delivery.Address, o.Delivery.Region, o.Delivery.Email)
    if err != nil {
        return err
    }

    // upsert payment
    _, err = tx.Exec(ctx, `
    INSERT INTO payment (order_uid, transaction, request_id, currency, provider, amount, payment_dt, bank, delivery_cost, goods_total, custom_fee)
    VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
    ON CONFLICT (order_uid) DO UPDATE SET
      transaction = EXCLUDED.transaction,
      request_id = EXCLUDED.request_id,
      currency = EXCLUDED.currency,
      provider = EXCLUDED.provider,
      amount = EXCLUDED.amount,
      payment_dt = EXCLUDED.payment_dt,
      bank = EXCLUDED.bank,
      delivery_cost = EXCLUDED.delivery_cost,
      goods_total = EXCLUDED.goods_total,
      custom_fee = EXCLUDED.custom_fee
    `, o.OrderUID, o.Payment.Transaction, o.Payment.RequestID, o.Payment.Currency, o.Payment.Provider, o.Payment.Amount, o.Payment.PaymentDT, o.Payment.Bank, o.Payment.DeliveryCost, o.Payment.GoodsTotal, o.Payment.CustomFee)
    if err != nil {
        return err
    }

    // Delete existing items for order and insert new (simpler than upsert for items)
    _, err = tx.Exec(ctx, `DELETE FROM items WHERE order_uid = $1`, o.OrderUID)
    if err != nil {
        return err
    }

    for _, it := range o.Items {
        _, err = tx.Exec(ctx, `
        INSERT INTO items (order_uid, chrt_id, track_number, price, rid, name, sale, size, total_price, nm_id, brand, status)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
        `, o.OrderUID, it.ChrtID, it.TrackNumber, it.Price, it.Rid, it.Name, it.Sale, it.Size, it.TotalPrice, it.NmID, it.Brand, it.Status)
        if err != nil {
            return err
        }
    }

    if err := tx.Commit(ctx); err != nil {
        return err
    }
    return nil
}

// LoadAllOrders loads all orders from DB into memory (for cache warmup)
func (db *DB) LoadAllOrders(ctx context.Context) (map[string]*Order, error) {
    orders := make(map[string]*Order)

    rows, err := db.Pool.Query(ctx, `SELECT order_uid, track_number, entry, locale, internal_signature, customer_id, delivery_service, shardkey, sm_id, date_created, oof_shard FROM orders`)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    for rows.Next() {
        var o Order
        var dateCreated time.Time
        err := rows.Scan(&o.OrderUID, &o.TrackNumber, &o.Entry, &o.Locale, &o.InternalSignature, &o.CustomerID, &o.DeliveryService, &o.Shardkey, &o.SmID, &dateCreated, &o.OofShard)
        if err != nil {
            return nil, err
        }
        o.DateCreated = dateCreated

        // load delivery
        var d Delivery
        err = db.Pool.QueryRow(ctx, `SELECT name, phone, zip, city, address, region, email FROM delivery WHERE order_uid = $1`, o.OrderUID).
            Scan(&d.Name, &d.Phone, &d.Zip, &d.City, &d.Address, &d.Region, &d.Email)
        if err == nil {
            o.Delivery = d
        }

        // load payment
        var p Payment
        err = db.Pool.QueryRow(ctx, `SELECT transaction, request_id, currency, provider, amount, payment_dt, bank, delivery_cost, goods_total, custom_fee FROM payment WHERE order_uid = $1`, o.OrderUID).
            Scan(&p.Transaction, &p.RequestID, &p.Currency, &p.Provider, &p.Amount, &p.PaymentDT, &p.Bank, &p.DeliveryCost, &p.GoodsTotal, &p.CustomFee)
        if err == nil {
            o.Payment = p
        }

        // load items
        itemRows, err := db.Pool.Query(ctx, `SELECT chrt_id, track_number, price, rid, name, sale, size, total_price, nm_id, brand, status FROM items WHERE order_uid = $1`, o.OrderUID)
        if err == nil {
            defer itemRows.Close()
            for itemRows.Next() {
                var it Item
                if err := itemRows.Scan(&it.ChrtID, &it.TrackNumber, &it.Price, &it.Rid, &it.Name, &it.Sale, &it.Size, &it.TotalPrice, &it.NmID, &it.Brand, &it.Status); err == nil {
                    o.Items = append(o.Items, it)
                }
            }
        }

        orders[o.OrderUID] = &o
    }
    return orders, nil
}

