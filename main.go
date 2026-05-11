package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/undeadops/webby/pkg/brews"
	"github.com/undeadops/webby/pkg/telemetry"
	"github.com/undeadops/webby/pkg/version"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

var (
	port          string
	delayShutdown bool
)

type middleware func(http.Handler) http.Handler
type middlewares []middleware

func (mws middlewares) apply(hdlr http.Handler) http.Handler {
	if len(mws) == 0 {
		return hdlr
	}
	return mws[1:].apply(mws[0](hdlr))
}

func (c *controller) shutdown(ctx context.Context, server *http.Server) context.Context {
	ctx, done := context.WithCancel(ctx)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		defer done()

		<-quit
		signal.Stop(quit)
		close(quit)

		atomic.StoreInt64(&c.healthy, 0)
		server.ErrorLog.Printf("Server is shutting down...\n")

		if c.delayShutdown {
			server.ErrorLog.Printf("Delaying shutdown for around 35 seconds")
			b, err := brews.GetBrews("san_diego")
			if err != nil {
				server.ErrorLog.Printf("Error Getting San Diego, %s", err)
			}
			fmt.Printf("%v\n", b)
			server.ErrorLog.Printf("Sleeping for 20 sec")
			time.Sleep(20 * time.Second)
			b, err = brews.GetBrews("denver")
			if err != nil {
				server.ErrorLog.Printf("Error Getting Denver, %s", err)
			}
			fmt.Printf("%v", b)
			server.ErrorLog.Printf("Sleeping for 15 sec")
			time.Sleep(15 * time.Second)
			b, err = brews.GetBrews("salt_lake_city")
			if err != nil {
				server.ErrorLog.Printf("Error Getting Salt Lake City, %s", err)
			}
			fmt.Printf("%v", b)
			server.ErrorLog.Printf("End of Dely")
		}

		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		server.SetKeepAlivesEnabled(false)
		if err := server.Shutdown(ctx); err != nil {
			server.ErrorLog.Fatalf("Could not gracefully shutdown the server: %s\n", err)
		}
	}()

	return ctx
}

type controller struct {
	logger        *log.Logger
	nextRequestID func() string
	healthy       int64
	delayShutdown bool
	webhookCount  metric.Int64Counter
}

type message struct {
	Message string
	Version string
}

func main() {
	flag.StringVar(&port, "port", ":5000", "Port to listen on")
	flag.BoolVar(&delayShutdown, "delay", false, "Delay shutdown by around 35 seconds")
	flag.Parse()

	logger := log.New(os.Stdout, "http: ", log.LstdFlags)
	logger.Printf("Server is starting...")

	otelShutdown, otelEnabled, err := telemetry.Init(context.Background(), "webby")
	if err != nil {
		logger.Fatalf("Could not initialize OpenTelemetry: %s\n", err)
	}
	if otelEnabled {
		logger.Printf("OpenTelemetry enabled")
	}

	webhookCount, err := otel.Meter("webby").Int64Counter(
		"webby.webhook.requests",
		metric.WithDescription("Number of requests to the /webhook endpoint"),
	)
	if err != nil {
		logger.Fatalf("Could not create webhook counter: %s\n", err)
	}

	c := &controller{
		logger:        logger,
		nextRequestID: func() string { return strconv.FormatInt(time.Now().UnixNano(), 36) },
		delayShutdown: delayShutdown,
		webhookCount:  webhookCount,
	}

	router := http.NewServeMux()
	router.HandleFunc("/", c.index)
	router.HandleFunc("/brews", c.brews)
	router.HandleFunc("/healthz", c.healthz)
	router.HandleFunc("/webhook", c.webhook)

	server := &http.Server{
		Addr:         port,
		Handler:      (middlewares{c.tracing, c.logging}).apply(router),
		ErrorLog:     logger,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  15 * time.Second,
	}
	ctx := c.shutdown(context.Background(), server)

	logger.Printf("Server is ready to handle requests at %q\n", port)
	atomic.StoreInt64(&c.healthy, time.Now().UnixNano())

	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		logger.Fatalf("Could not listen on %q: %s\n", port, err)
	}
	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := otelShutdown(shutdownCtx); err != nil {
		logger.Printf("Error shutting down OpenTelemetry: %s\n", err)
	}

	logger.Printf("Server stopped\n")
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, _ := json.Marshal(payload)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}

func (c *controller) index(w http.ResponseWriter, req *http.Request) {
	if req.URL.Path != "/" {
		http.NotFound(w, req)
		return
	}

	m := message{
		Message: "Hello, World!",
		Version: fmt.Sprintf("commit: %s, build time: %s, release: %s", version.Commit, version.BuildTime, version.Release),
	}
	requestDump, err := httputil.DumpRequest(req, true)
	if err != nil {
		fmt.Println(err)
	}
	c.logger.Printf("%s", string(requestDump))

	respondWithJSON(w, http.StatusCreated, m)
}

func (c *controller) healthz(w http.ResponseWriter, req *http.Request) {
	if h := atomic.LoadInt64(&c.healthy); h == 0 {
		w.WriteHeader(http.StatusServiceUnavailable)
	} else {
		fmt.Fprintf(w, "uptime: %s\n", time.Since(time.Unix(0, h)))
	}
}

func (c *controller) brews(w http.ResponseWriter, req *http.Request) {
	brews, err := brews.GetBrews("san_diego")
	if err != nil {
		c.logger.Printf("Error getting brews: %v", err)
	}
	respondWithJSON(w, http.StatusOK, brews)
}

func (c *controller) webhook(w http.ResponseWriter, req *http.Request) {
	c.webhookCount.Add(req.Context(), 1)

	if req.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	queryParams := req.URL.Query()
	codeParm := queryParams.Get("code")
	if codeParm != "" {
		codeInt, err := strconv.Atoi(codeParm)
		if err == nil && codeInt >= 100 && codeInt <= 599 {
			http.Error(w, fmt.Sprintf("Returning status code %d", codeInt), codeInt)
			return
		}
	}

	body, err := io.ReadAll(req.Body)
	if err != nil {
		http.Error(w, "Error reading request body", http.StatusInternalServerError)
		return
	}
	defer req.Body.Close()

	headers := formatRequest(req)
	fmt.Printf("\n==========\nHEADERS:\n%v", headers)
	fmt.Printf("BODY:%s\n", string(body))
	fmt.Println("==========")
	respondWithJSON(w, http.StatusOK, "Received Webhook")
}

func (c *controller) logging(hdlr http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		defer func(start time.Time) {
			requestID := w.Header().Get("X-Request-Id")
			if requestID == "" {
				requestID = "unknown"
			}
			c.logger.Println(requestID, req.Method, req.URL.Path, req.RemoteAddr, req.UserAgent(), time.Since(start))
		}(time.Now())
		hdlr.ServeHTTP(w, req)
	})
}

func (c *controller) tracing(hdlr http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		requestID := req.Header.Get("X-Request-Id")
		if requestID == "" {
			requestID = c.nextRequestID()
		}
		w.Header().Set("X-Request-Id", requestID)
		hdlr.ServeHTTP(w, req)
	})
}

func formatRequest(r *http.Request) string {
	// Create return string
	var request []string
	// Add the request string
	url := fmt.Sprintf("%v %v %v", r.Method, r.URL, r.Proto)
	request = append(request, url)
	// Add the host
	request = append(request, fmt.Sprintf("Host: %v", r.Host))
	// Loop through headers
	for name, headers := range r.Header {
		name = strings.ToLower(name)
		for _, h := range headers {
			request = append(request, fmt.Sprintf("%v: %v", name, h))
		}
	}

	// If this is a POST, add post data
	if r.Method == "POST" {
		r.ParseForm()
		request = append(request, "\n")
		request = append(request, r.Form.Encode())
	}
	// Return the request as a string
	return strings.Join(request, "\n")
}
