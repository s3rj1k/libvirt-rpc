package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/semrush/zenrpc"
	"golang.org/x/time/rate"
)

var limiter = rate.NewLimiter(5, 10)

// http://www.alexedwards.net/blog/how-to-rate-limit-http-requests
func limitHandler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !limiter.Allow() {
			http.Error(w, http.StatusText(429), http.StatusTooManyRequests)
			return
		}
		h.ServeHTTP(w, r)
	})
}

func loggingHandler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		info.Printf("remote_addr=%s, method=%s, request_URI=%s", r.RemoteAddr, r.Method, r.RequestURI)
		h.ServeHTTP(w, r)
	})
}

func logger() zenrpc.MiddlewareFunc {
	return func(h zenrpc.InvokeFunc) zenrpc.InvokeFunc {
		return func(ctx context.Context, method string, params json.RawMessage) zenrpc.Response {
			start, ip := time.Now(), "<nil>"
			if req, ok := zenrpc.RequestFromContext(ctx); ok && req != nil {

				var in bytes.Buffer

				ip = req.RemoteAddr
				id := getReqIDFromContext(ctx)

				err := json.Compact(&in, params)
				if err != nil {
					fail.Printf("failed to compact JSON, %s", err.Error())
				}

				info.Printf("%sip=%s, method=%s.%s, params=%s", id, ip, zenrpc.NamespaceFromContext(ctx), method, in.String())
			}

			r := h(ctx, method, params)

			var out bytes.Buffer

			id := getReqIDFromContext(ctx)

			if r.Result != nil {
				err := json.Compact(&out, *r.Result)
				if err != nil {
					fail.Printf("failed to compact JSON, %s", err.Error())
				}
			}

			if r.Error != nil {
				fail.Printf("%sduration=%v, response=%s, err=%s", id, time.Since(start), out.String(), r.Error)
				return r
			}

			info.Printf("%sduration=%v, response=%s", id, time.Since(start), out.String())
			return r
		}
	}
}
