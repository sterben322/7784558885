// ══════════════════════════════════════════════════════════════════
//  ВСТАВИТЬ В main.go — в блок регистрации защищённых маршрутов API
//  (туда же, где зарегистрированы /api/resumes, /api/jobs и т.д.)
//
//  Пример (если используется gorilla/mux):
//
//  auth := r.PathPrefix("/api").Subrouter()
//  auth.Use(middleware.AuthRequired)
//  ... существующие маршруты ...
//
//  // ── Events ──────────────────────────────────────
//  auth.HandleFunc("/events",                  handlers.EventsList).Methods("GET")
//  auth.HandleFunc("/events",                  handlers.EventCreate).Methods("POST")
//  auth.HandleFunc("/events/stats",            handlers.EventsStatsHandler).Methods("GET")
//  auth.HandleFunc("/events/my",               handlers.EventsMy).Methods("GET")
//  auth.HandleFunc("/events/{id}",             handlers.EventGet).Methods("GET")
//  auth.HandleFunc("/events/{id}/register",    handlers.EventRegister).Methods("POST")
//  auth.HandleFunc("/events/{id}/register",    handlers.EventUnregister).Methods("DELETE")
//  auth.HandleFunc("/events/{id}/view",        handlers.EventView).Methods("POST")
//
//  // ── Static pages (если ещё не добавлены) ────────
//  // Сервер уже отдаёт всё из web/ через FileServer,
//  // поэтому events.html и exhibitions.html работают автоматически.
// ══════════════════════════════════════════════════════════════════

// Если в проекте НЕ используется gorilla/mux, а маршруты регистрируются
// через стандартный http.ServeMux — используйте такой вариант:
//
//  mux.HandleFunc("/api/events", withAuth(eventsDispatcher))
//
//  func eventsDispatcher(w http.ResponseWriter, r *http.Request) {
//      id := pathEventID(r)
//      suffix := pathSuffix(r)
//      switch {
//      case r.Method == "GET"  && id == "" && suffix == "stats": handlers.EventsStatsHandler(w, r)
//      case r.Method == "GET"  && id == "" && suffix == "my":    handlers.EventsMy(w, r)
//      case r.Method == "GET"  && id == "":                       handlers.EventsList(w, r)
//      case r.Method == "POST" && id == "":                       handlers.EventCreate(w, r)
//      case r.Method == "GET"  && id != "":                       handlers.EventGet(w, r)
//      case r.Method == "POST" && suffix == "register":           handlers.EventRegister(w, r)
//      case r.Method == "DELETE" && suffix == "register":         handlers.EventUnregister(w, r)
//      case r.Method == "POST" && suffix == "view":               handlers.EventView(w, r)
//      default: http.NotFound(w, r)
//      }
//  }
