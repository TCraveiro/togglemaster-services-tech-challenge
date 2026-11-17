package main

import (
	"net/http"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// instrumentHandler envolve o router com o otelhttp: cria um span SERVER por requisição
// (continuando o trace recebido no header traceparent) e gera a métrica
// http.server.request.duration (taxa de requisições, latência e status code).
// O /health fica de fora para não poluir traces e métricas com as probes do Kubernetes.
func instrumentHandler(h http.Handler, service string) http.Handler {
	return otelhttp.NewHandler(h, service,
		otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
			return r.Method + " " + r.URL.Path
		}),
		otelhttp.WithFilter(func(r *http.Request) bool {
			return r.URL.Path != "/health"
		}),
	)
}
