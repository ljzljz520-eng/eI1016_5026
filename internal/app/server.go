package app

import (
	"encoding/json"
	"html/template"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"buildingops/internal/audit"
	"buildingops/internal/auth"
	"buildingops/internal/operations"
	"buildingops/internal/session"
	webui "buildingops/web"
)

const sessionCookie = "buildingops_session"

type Options struct {
	FailureBarrier *auth.Barrier
}

type Server struct {
	mux        *http.ServeMux
	auth       *auth.Service
	sessions   *session.Store
	audit      audit.Log
	operations operations.Repository
	loginPage  *template.Template
	appPage    *template.Template
}

type loginPageData struct {
	Message string
}

func NewServer(options Options) *Server {
	const username = "ops.admin"
	accounts := auth.NewMemoryAccountRepository([]auth.Account{
		auth.NewAccount(username, "facility-123"),
	})
	protection := auth.NewMemoryProtectionRepository([]string{username}, 2, options.FailureBarrier)
	server := &Server{
		mux:        http.NewServeMux(),
		auth:       auth.NewService(accounts, protection),
		sessions:   session.NewStore("building-operations-local-fixture"),
		audit:      audit.NewMemoryLog(time.Date(2026, time.August, 15, 8, 0, 0, 0, time.UTC)),
		operations: fixedOperations(),
		loginPage:  template.Must(template.ParseFS(webui.Files, "src/login.html")),
		appPage:    template.Must(template.ParseFS(webui.Files, "src/app.html")),
	}
	server.routes()
	return server
}

func fixedOperations() operations.Repository {
	return operations.NewMemoryRepository([]operations.AssetStatus{
		{Kind: "elevator", Name: "Elevators", State: "operational", Detail: "8 of 8 cars in service", UpdatedAt: "08:42", OpenTickets: 0},
		{Kind: "hvac", Name: "Air conditioning", State: "attention", Detail: "East wing filter due", UpdatedAt: "08:39", OpenTickets: 2},
		{Kind: "access", Name: "Access control", State: "operational", Detail: "24 of 24 doors online", UpdatedAt: "08:41", OpenTickets: 1},
	})
}

func (s *Server) routes() {
	assets, err := fs.Sub(webui.Files, "src")
	if err != nil {
		panic(err)
	}
	staticHandler := http.StripPrefix("/assets/", http.FileServer(http.FS(assets)))
	s.mux.Handle("GET /assets/styles.css", staticHandler)
	s.mux.Handle("GET /assets/app.js", staticHandler)
	s.mux.HandleFunc("GET /", s.handleHome)
	s.mux.HandleFunc("POST /login", s.handleLogin)
	s.mux.HandleFunc("POST /logout", s.handleLogout)
	s.mux.HandleFunc("GET /api/status", s.handleStatus)
	s.mux.HandleFunc("GET /api/audit", s.handleAudit)
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Referrer-Policy", "same-origin")
	s.mux.ServeHTTP(w, r)
}

func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	username, ok := s.currentUsername(r)
	if !ok {
		s.renderLogin(w, http.StatusOK, "")
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.appPage.ExecuteTemplate(w, "app.html", map[string]string{"Username": username}); err != nil {
		http.Error(w, "unable to render application", http.StatusInternalServerError)
	}
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		s.renderLogin(w, http.StatusBadRequest, "Invalid sign-in request")
		return
	}
	username := strings.TrimSpace(r.FormValue("username"))
	result := s.auth.Login(username, r.FormValue("password"))
	s.audit.Append(username, "login", string(result))
	switch result {
	case auth.LoginAllowed:
		token := s.sessions.Create(username)
		http.SetCookie(w, &http.Cookie{
			Name:     sessionCookie,
			Value:    token,
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteStrictMode,
		})
		http.Redirect(w, r, "/", http.StatusSeeOther)
	case auth.LoginLocked:
		s.renderLogin(w, http.StatusLocked, "Account locked after repeated sign-in failures")
	default:
		s.renderLogin(w, http.StatusUnauthorized, "Username or password is incorrect")
	}
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(sessionCookie)
	if err == nil {
		if username, ok := s.sessions.Username(cookie.Value); ok {
			s.audit.Append(username, "logout", "completed")
		}
		s.sessions.Delete(cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAPIUser(w, r); !ok {
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"assets": s.operations.List()})
}

func (s *Server) handleAudit(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAPIUser(w, r); !ok {
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"entries": s.audit.List()})
}

func (s *Server) currentUsername(r *http.Request) (string, bool) {
	cookie, err := r.Cookie(sessionCookie)
	if err != nil {
		return "", false
	}
	return s.sessions.Username(cookie.Value)
}

func (s *Server) requireAPIUser(w http.ResponseWriter, r *http.Request) (string, bool) {
	username, ok := s.currentUsername(r)
	if !ok {
		s.writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "authentication required"})
		return "", false
	}
	return username, true
}

func (s *Server) renderLogin(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := s.loginPage.ExecuteTemplate(w, "login.html", loginPageData{Message: message}); err != nil {
		return
	}
}

func (s *Server) writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
