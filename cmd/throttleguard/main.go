package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/example/throttleguard/internal/config"
	"github.com/example/throttleguard/internal/limiter"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	settings, err := config.Load()
	if err != nil {
		logger.Error("load configuration", "error", err)
		os.Exit(1)
	}

	backendURL, err := url.Parse(settings.BackendURL)
	if err != nil {
		logger.Error("parse backend URL", "error", err)
		os.Exit(1)
	}

	requestLimiter, err := newLimiter(settings)
	if err != nil {
		logger.Error("create limiter", "error", err)
		os.Exit(1)
	}

	proxy := httputil.NewSingleHostReverseProxy(backendURL)
	handler := newHandler(proxy, requestLimiter)
	server := &http.Server{
		Addr:              settings.Address,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	serverErrors := make(chan error, 1)
	go func() {
		logger.Info("server listening", "address", settings.Address, "backend", settings.BackendURL)
		serverErrors <- server.ListenAndServe()
	}()

	shutdownContext, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	select {
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server stopped", "error", err)
			os.Exit(1)
		}
	case <-shutdownContext.Done():
		shutdownDeadline, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownDeadline); err != nil {
			logger.Error("graceful shutdown failed", "error", err)
			os.Exit(1)
		}
		logger.Info("server stopped")
	}
}

func newLimiter(settings config.Settings) (limiter.RateLimiter, error) {
	if settings.RedisAddr != "" {
		redisLimiter, err := limiter.NewRedis(settings.RedisAddr, settings.RateLimit, settings.RateWindow)
		if err != nil {
			return nil, err
		}
		return redisLimiter, nil
	}
	return limiter.New(settings.RateLimit, settings.RateWindow), nil
}

func newHandler(proxy http.Handler, requestLimiter limiter.RateLimiter) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(response http.ResponseWriter, request *http.Request) {
		response.WriteHeader(http.StatusOK)
		_, _ = response.Write([]byte("ok\n"))
	})
	mux.Handle("/", rateLimitMiddleware(requestLimiter, proxy))
	return mux
}

func rateLimitMiddleware(requestLimiter limiter.RateLimiter, next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		clientIP := limiter.ClientIP(request)
		if !requestLimiter.Allow(clientIP) {
			response.Header().Set("Retry-After", "1")
			http.Error(response, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(response, request)
	})
}
