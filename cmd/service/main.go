package main

import (
    "context"
    "flag"
    "log"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/example/order-service/internal"

)

func main() {
    var (
        dsn       = flag.String("dsn", "postgres://demo_user:demo_pass@postgres:5432/demo_orders", "Postgres DSN")
        natsURL   = flag.String("nats", "nats://nats-streaming:4222", "NATS URL")
        clusterID = flag.String("cluster", "test-cluster", "NATS Streaming cluster ID")
        clientID  = flag.String("client", "order-svc-1", "NATS client ID")
        channel   = flag.String("channel", "orders", "NATS channel to subscribe")
        durable   = flag.String("durable", "order-durable", "NATS durable name")
        httpAddr  = flag.String("http", ":8080", "HTTP listen address")
    )
    flag.Parse()

    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    // DB
    db, err := internal.NewDB(ctx, *dsn)
    if err != nil {
        log.Fatalf("failed to connect db: %v", err)
    }
    defer db.Close()

    // cache and warmup
    cache := internal.NewCache()
    log.Println("warming cache from DB...")
    all, err := db.LoadAllOrders(ctx)
    if err != nil {
        log.Printf("warning: failed to load initial orders: %v", err)
    } else {
        cache.LoadAll(all)
        log.Printf("loaded %d orders into cache", len(all))
    }

    // NATS consumer
    natsCons, err := internal.NewNatsConsumer(*clusterID, *clientID, *natsURL, *channel, *durable, db, cache)
    if err != nil {
        log.Fatalf("failed to start nats consumer: %v", err)
    }
    defer natsCons.Close()

    // start consumer in background
    go func() {
        if err := natsCons.Start(ctx); err != nil {
            log.Printf("nats consumer stopped: %v", err)
        }
    }()

    // http server
    srv := internal.NewServer(db, cache)
    server := &http.Server{
        Addr: *httpAddr,
        Handler: srv,
        ReadTimeout:  10 * time.Second,
        WriteTimeout: 10 * time.Second,
        IdleTimeout:  60 * time.Second,
    }

    go func() {
        log.Printf("http server listening on %s", *httpAddr)
        if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            log.Fatalf("http server error: %v", err)
        }
    }()

    // graceful shutdown on SIGINT/SIGTERM
    stop := make(chan os.Signal, 1)
    signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
    <-stop
    log.Println("shutting down...")

    cancel() // stop consumer

    ctxShutdown, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancelShutdown()
    if err := server.Shutdown(ctxShutdown); err != nil {
        log.Printf("http shutdown error: %v", err)
    }
    log.Println("bye")
}
