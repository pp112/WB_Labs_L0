package internal

import (
    "encoding/json"
    "html/template"
    "log"
    "net/http"

    "github.com/gorilla/mux"
)

type Server struct {
    DB    *DB
    Cache *Cache
    Router *mux.Router
    tmpl  *template.Template
}

func NewServer(db *DB, cache *Cache) *Server {
    s := &Server{
        DB: db,
        Cache: cache,
        Router: mux.NewRouter(),
    }
    s.routes()
    return s
}

func (s *Server) routes() {
    s.Router.HandleFunc("/order/{id}", s.handleGetOrder).Methods("GET")
    s.Router.HandleFunc("/ui", s.handleUI).Methods("GET")
    s.Router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("./web/"))))
}

func (s *Server) handleGetOrder(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    id := vars["id"]
    if id == "" {
        http.Error(w, "missing id", http.StatusBadRequest)
        return
    }
    if o, ok := s.Cache.Get(id); ok {
        w.Header().Set("Content-Type", "application/json")
        _ = json.NewEncoder(w).Encode(o)
        return
    }

    // not found in cache: try to load from DB and populate cache
    // simple approach: load all and look up (could query specific)
    all, err := s.DB.LoadAllOrders(r.Context())
    if err == nil {
        s.Cache.LoadAll(all)
        if o, ok := s.Cache.Get(id); ok {
            w.Header().Set("Content-Type", "application/json")
            _ = json.NewEncoder(w).Encode(o)
            return
        }
    } else {
        log.Printf("failed to load from db: %v", err)
    }

    http.Error(w, "order not found", http.StatusNotFound)
}

func (s *Server) handleUI(w http.ResponseWriter, r *http.Request) {
    tmpl, err := template.ParseFiles("./web/index.html")
    if err != nil {
        http.Error(w, "template error", http.StatusInternalServerError)
        return
    }
    tmpl.Execute(w, nil)
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    s.Router.ServeHTTP(w, r)
}
