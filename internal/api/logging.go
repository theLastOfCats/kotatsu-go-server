package api

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"time"
)

// LoggingMiddleware logs all HTTP requests with request/response bodies
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Read and log request body
		var requestBody []byte
		if r.Body != nil {
			requestBody, _ = io.ReadAll(r.Body)
			r.Body = io.NopCloser(bytes.NewBuffer(requestBody)) // Restore body for handler
		}

		log.Printf("[%s] %s %s", r.Method, r.URL.Path, r.RemoteAddr)
		if len(requestBody) > 0 && len(requestBody) < 10000 { // Only log if < 10KB
			log.Printf("  Request Body: %s", string(requestBody))
		}

		// Create a response writer wrapper to capture status code and body
		wrapped := &responseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
			body:           &bytes.Buffer{},
		}

		// Call next handler
		next.ServeHTTP(wrapped, r)

		// Log response
		duration := time.Since(start)
		log.Printf("[%s] %s - %d (%v)", r.Method, r.URL.Path, wrapped.statusCode, duration)
		if wrapped.body.Len() > 0 && wrapped.body.Len() < 10000 { // Only log if < 10KB
			log.Printf("  Response Body: %s", wrapped.body.String())
		}
	})
}

// responseWriter wraps http.ResponseWriter to capture status code and body
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	body       *bytes.Buffer
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	rw.body.Write(b) // Capture response body
	return rw.ResponseWriter.Write(b)
}
