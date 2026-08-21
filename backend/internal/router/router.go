package router

import (
	"log/slog"
	"net/http"

	"gojira/internal/auth"
	"gojira/internal/board"
	"gojira/internal/comment"
	"gojira/internal/config"
	"gojira/internal/gantt"
	"gojira/internal/health"
	"gojira/internal/issue"
	"gojira/internal/middleware"
	"gojira/internal/project"
	"gojira/internal/sprint"
	"gojira/internal/stats"
	"gojira/internal/trigger"
	"gojira/internal/workflow"

	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"
)

type Deps struct {
	DB     *sqlx.DB
	Cfg    *config.Config
	Log    *slog.Logger
	Engine *workflow.Engine
}

func New(d Deps) http.Handler {
	a := &auth.Service{DB: d.DB, Secret: []byte(d.Cfg.JWTSecret)}
	proj := &project.Service{DB: d.DB}
	iss := &issue.Service{DB: d.DB, Engine: d.Engine}
	sp := &sprint.Service{DB: d.DB}
	bd := &board.Service{DB: d.DB, Engine: d.Engine}
	gt := &gantt.Service{DB: d.DB}
	cm := &comment.Service{DB: d.DB}
	st := &stats.Service{DB: d.DB, Log: d.Log}
	tr := &trigger.API{DB: d.DB}
	hh := &health.Handler{DB: d.DB, Cfg: d.Cfg}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Recover(d.Log))
	r.Use(middleware.CORS)

	r.Get("/api/health", hh.ServeHTTP)
	r.Get("/api/v1/health", hh.ServeHTTP)

	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/auth/register", a.Register)
		r.Post("/auth/login", a.Login)
		r.Post("/auth/refresh", a.Refresh)

		r.Group(func(r chi.Router) {
			r.Use(middleware.JWT(a))
			r.Get("/auth/me", a.Me)

			r.Get("/projects", proj.List)
			r.Post("/projects", proj.Create)

			r.Route("/projects/{id}", func(r chi.Router) {
				r.Use(middleware.RequireProject(d.DB))
				r.Get("/", proj.Get)
				r.Patch("/", proj.Patch)
				r.Get("/members", proj.ListMembers)
				r.Post("/members", proj.AddMember)
				r.Delete("/members/{userID}", proj.RemoveMember)
				r.Get("/issues", iss.List)
				r.Post("/issues", iss.Create)
				r.Get("/board", bd.Get)
				r.Get("/sprints", sp.List)
				r.Post("/sprints", sp.Create)
				r.Get("/gantt", gt.View)
				r.Get("/velocity", st.Velocity)
				r.Get("/bug-stats", st.BugStats)
				r.Get("/triggers", tr.List)
				r.Get("/trigger-executions", tr.Executions)
			})

			r.Route("/issues/{id}", func(r chi.Router) {
				r.Use(middleware.RequireIssueProject(d.DB))
				r.Get("/", iss.Get)
				r.Patch("/", iss.Patch)
				r.Post("/transition", iss.Transition)
				r.Get("/history", iss.History)
				r.Get("/comments", cm.List)
				r.Post("/comments", cm.Create)
				r.Get("/dependencies", iss.ListDeps)
				r.Post("/dependencies", iss.AddDep)
				r.Delete("/dependencies/{depID}", iss.DeleteDep)
				r.Patch("/rank", iss.Rank)
			})

			r.Route("/sprints/{id}", func(r chi.Router) {
				r.Use(middleware.RequireSprintProject(d.DB))
				r.Post("/start", sp.Start)
				r.Post("/close", sp.Close)
				r.Post("/issues", sp.AssignIssues)
				r.Delete("/issues/{issueID}", sp.UnassignIssue)
				r.Get("/burndown", st.Burndown)
				r.Get("/progress", st.Progress)
			})
		})
	})
	return r
}
