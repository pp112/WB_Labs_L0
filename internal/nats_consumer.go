package internal

import (
    "context"
    "encoding/json"
    "log"
    "time"

    stan "github.com/nats-io/stan.go"
)

type NatsConsumer struct {
    Conn       stan.Conn
    Channel    string
    DB         *DB
    Cache      *Cache
    Durable    string
    ClientID   string
    ClusterID  string
    NatsURL    string
}

func NewNatsConsumer(clusterID, clientID, natsURL, channel, durable string, db *DB, cache *Cache) (*NatsConsumer, error) {
    sc, err := stan.Connect(clusterID, clientID, stan.NatsURL(natsURL))
    if err != nil {
        return nil, err
    }
    return &NatsConsumer{
        Conn:      sc,
        Channel:   channel,
        DB:        db,
        Cache:     cache,
        Durable:   durable,
        ClientID:  clientID,
        ClusterID: clusterID,
        NatsURL:   natsURL,
    }, nil
}

func (nc *NatsConsumer) Close() {
    if nc.Conn != nil {
        nc.Conn.Close()
    }
}

func (nc *NatsConsumer) Start(ctx context.Context) error {
    // Subscribe with manual ack, durable
    _, err := nc.Conn.Subscribe(nc.Channel, func(m *stan.Msg) {
        var o Order
        if err := json.Unmarshal(m.Data, &o); err != nil {
            log.Printf("failed to unmarshal message: %v", err)
            // do not ack to allow redelivery? but we'll ack to avoid stuck queue
            _ = m.Ack()
            return
        }

        // Convert date_created if it is string? Our model expects time.Time; assume JSON gives RFC3339 string
        // If date parsing was required, ensure models use string and parse. Here assume correct type.

        // Write to DB
        ctx2, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()
        if err := nc.DB.UpsertOrder(ctx2, &o); err != nil {
            log.Printf("db upsert failed for %s: %v", o.OrderUID, err)
            // optionally do not ack to retry; but ack to avoid infinite loop
            _ = m.Ack()
            return
        }

        // Update in-memory cache
        nc.Cache.Set(o.OrderUID, &o)

        // Ack message
        if err := m.Ack(); err != nil {
            // If ack not supported (subscription not configured), ignore
            log.Printf("ack failed: %v", err)
        }
        log.Printf("processed order %s", o.OrderUID)
    }, stan.DurableName(nc.Durable), stan.SetManualAckMode(), stan.StartWithLastReceived()) // ensure we get new messages
    if err != nil {
        return err
    }

    // keep running until ctx done
    <-ctx.Done()
    return nil
}
