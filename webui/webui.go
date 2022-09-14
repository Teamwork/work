package webui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"

	"github.com/braintree/manners"
	"github.com/gocraft/web"
	"github.com/gomodule/redigo/redis"
	work "github.com/teamwork/work/v2"
	"github.com/teamwork/work/v2/webui/internal/assets"
)

// Server implements an HTTP server which exposes a JSON API to view and manage gocraft/work items.
type Server struct {
	pool     *redis.Pool
	hostPort string
	server   *manners.GracefulServer
	wg       sync.WaitGroup
	router   *web.Router
}

type context struct {
	*Server
}

// NewServer creates and returns a new server. The hostPort param is the address to bind on to expose the API.
func NewServer(pool *redis.Pool, hostPort string) *Server {
	router := web.New(context{})
	server := &Server{
		pool:     pool,
		hostPort: hostPort,
		server:   manners.NewWithServer(&http.Server{Addr: hostPort, Handler: router}),
		router:   router,
	}

	router.Middleware(func(c *context, rw web.ResponseWriter, r *web.Request, next web.NextMiddlewareFunc) {
		c.Server = server
		next(rw, r)
	})
	router.Middleware(func(rw web.ResponseWriter, r *web.Request, next web.NextMiddlewareFunc) {
		rw.Header().Set("Content-Type", "application/json; charset=utf-8")
		next(rw, r)
	})
	router.Get("/:namespace/queues", (*context).queues)
	router.Get("/:namespace/worker_pools", (*context).workerPools)
	router.Get("/:namespace/busy_workers", (*context).busyWorkers)
	router.Get("/:namespace/retry_jobs", (*context).retryJobs)
	router.Get("/:namespace/scheduled_jobs", (*context).scheduledJobs)
	router.Get("/:namespace/dead_jobs", (*context).deadJobs)
	router.Post("/:namespace/delete_dead_job/:died_at:\\d.*/:job_id", (*context).deleteDeadJob)
	router.Post("/:namespace/retry_dead_job/:died_at:\\d.*/:job_id", (*context).retryDeadJob)
	router.Post("/:namespace/delete_all_dead_jobs", (*context).deleteAllDeadJobs)
	router.Post("/:namespace/retry_all_dead_jobs", (*context).retryAllDeadJobs)

	router.Get("/", func(c *context, rw web.ResponseWriter, req *web.Request) {
		rw.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintln(rw, "<h2>Welcome to workwebui.</h2>")
		fmt.Fprintln(rw, "<h4>Please provide a namespace in the url.</h4>")
		fmt.Fprintf(rw, "<h4>Example: <a href='http://localhost%s/ns/'>http://localhost%s/ns/</a></h4>",
			hostPort, hostPort)
	})

	//
	// Build the HTML page:
	//
	assetRouter := router.Subrouter(context{}, "")
	assetRouter.Get("/:namespace", func(c *context, rw web.ResponseWriter, req *web.Request) {
		rw.Header().Set("Content-Type", "text/html; charset=utf-8")
		b, err := assets.Asset("index.html")
		if err != nil {
			rw.WriteHeader(http.StatusNotFound)
			return
		}
		rw.Write(b)
	})
	assetRouter.Get("/work.js", func(c *context, rw web.ResponseWriter, req *web.Request) {
		rw.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		b, err := assets.Asset("work.js")
		if err != nil {
			rw.WriteHeader(http.StatusNotFound)
			return
		}
		rw.Write(b)
	})

	return server
}

// Start starts the server listening for requests on the hostPort specified in NewServer.
func (w *Server) Start() {
	w.wg.Add(1)
	go func(w *Server) {
		w.server.ListenAndServe()
		w.wg.Done()
	}(w)
}

// Stop stops the server and blocks until it has finished.
func (w *Server) Stop() {
	w.server.Close()
	w.wg.Wait()
}

func (c *context) queues(rw web.ResponseWriter, r *web.Request) {
	nsclient := work.NewClient(r.PathParams["namespace"], c.pool)
	response, err := nsclient.Queues()
	render(rw, response, err)
}

func (c *context) workerPools(rw web.ResponseWriter, r *web.Request) {
	nsclient := work.NewClient(r.PathParams["namespace"], c.pool)
	response, err := nsclient.WorkerPoolHeartbeats()
	render(rw, response, err)
}

func (c *context) busyWorkers(rw web.ResponseWriter, r *web.Request) {
	nsclient := work.NewClient(r.PathParams["namespace"], c.pool)
	observations, err := nsclient.WorkerObservations()
	if err != nil {
		renderError(rw, err)
		return
	}

	var busyObservations []*work.WorkerObservation
	for _, ob := range observations {
		if ob.IsBusy {
			busyObservations = append(busyObservations, ob)
		}
	}

	render(rw, busyObservations, err)
}

func (c *context) retryJobs(rw web.ResponseWriter, r *web.Request) {
	nsclient := work.NewClient(r.PathParams["namespace"], c.pool)
	page, err := parsePage(r)
	if err != nil {
		renderError(rw, err)
		return
	}

	jobs, count, err := nsclient.RetryJobs(page)
	if err != nil {
		renderError(rw, err)
		return
	}

	response := struct {
		Count int64            `json:"count"`
		Jobs  []*work.RetryJob `json:"jobs"`
	}{Count: count, Jobs: jobs}

	render(rw, response, err)
}

func (c *context) scheduledJobs(rw web.ResponseWriter, r *web.Request) {
	nsclient := work.NewClient(r.PathParams["namespace"], c.pool)
	page, err := parsePage(r)
	if err != nil {
		renderError(rw, err)
		return
	}

	jobs, count, err := nsclient.ScheduledJobs(page)
	if err != nil {
		renderError(rw, err)
		return
	}

	response := struct {
		Count int64                `json:"count"`
		Jobs  []*work.ScheduledJob `json:"jobs"`
	}{Count: count, Jobs: jobs}

	render(rw, response, err)
}

func (c *context) deadJobs(rw web.ResponseWriter, r *web.Request) {
	nsclient := work.NewClient(r.PathParams["namespace"], c.pool)
	page, err := parsePage(r)
	if err != nil {
		renderError(rw, err)
		return
	}

	jobs, count, err := nsclient.DeadJobs(page)
	if err != nil {
		renderError(rw, err)
		return
	}

	response := struct {
		Count int64           `json:"count"`
		Jobs  []*work.DeadJob `json:"jobs"`
	}{Count: count, Jobs: jobs}

	render(rw, response, err)
}

func (c *context) deleteDeadJob(rw web.ResponseWriter, r *web.Request) {
	nsclient := work.NewClient(r.PathParams["namespace"], c.pool)
	diedAt, err := strconv.ParseInt(r.PathParams["died_at"], 10, 64)
	if err != nil {
		renderError(rw, err)
		return
	}

	err = nsclient.DeleteDeadJob(diedAt, r.PathParams["job_id"])

	render(rw, map[string]string{"status": "ok"}, err)
}

func (c *context) retryDeadJob(rw web.ResponseWriter, r *web.Request) {
	nsclient := work.NewClient(r.PathParams["namespace"], c.pool)
	diedAt, err := strconv.ParseInt(r.PathParams["died_at"], 10, 64)
	if err != nil {
		renderError(rw, err)
		return
	}

	err = nsclient.RetryDeadJob(diedAt, r.PathParams["job_id"])

	render(rw, map[string]string{"status": "ok"}, err)
}

func (c *context) deleteAllDeadJobs(rw web.ResponseWriter, r *web.Request) {
	nsclient := work.NewClient(r.PathParams["namespace"], c.pool)
	err := nsclient.DeleteAllDeadJobs()
	render(rw, map[string]string{"status": "ok"}, err)
}

func (c *context) retryAllDeadJobs(rw web.ResponseWriter, r *web.Request) {
	nsclient := work.NewClient(r.PathParams["namespace"], c.pool)
	err := nsclient.RetryAllDeadJobs()
	render(rw, map[string]string{"status": "ok"}, err)
}

func render(rw web.ResponseWriter, jsonable interface{}, err error) {
	if err != nil {
		renderError(rw, err)
		return
	}

	jsonData, err := json.MarshalIndent(jsonable, "", "\t")
	if err != nil {
		renderError(rw, err)
		return
	}
	rw.Write(jsonData)
}

func renderError(rw http.ResponseWriter, err error) {
	rw.WriteHeader(500)
	fmt.Fprintf(rw, `{"error": "%s"}`, err.Error())
}

func parsePage(r *web.Request) (uint, error) {
	err := r.ParseForm()
	if err != nil {
		return 0, err
	}

	pageStr := r.Form.Get("page")
	if pageStr == "" {
		pageStr = "1"
	}

	page, err := strconv.ParseUint(pageStr, 10, 0)
	return uint(page), err
}
