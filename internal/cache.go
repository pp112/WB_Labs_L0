package internal

import "sync"

type Cache struct {
    mu    sync.RWMutex
    items map[string]*Order
}

func NewCache() *Cache {
    return &Cache{
        items: make(map[string]*Order),
    }
}

func (c *Cache) Get(id string) (*Order, bool) {
    c.mu.RLock()
    defer c.mu.RUnlock()
    o, ok := c.items[id]
    return o, ok
}

func (c *Cache) Set(id string, o *Order) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.items[id] = o
}

func (c *Cache) LoadAll(initial map[string]*Order) {
    c.mu.Lock()
    defer c.mu.Unlock()
    for k, v := range initial {
        c.items[k] = v
    }
}
