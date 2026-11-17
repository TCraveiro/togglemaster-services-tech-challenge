package main

import (
	"encoding/json"
	"log"
	"net/http"
	"sync/atomic"
)

// Injeção de falha (chaos) para a demonstração de Self-Healing.
// Quando ativado, o auth-service passa a responder HTTP 500 em todas as rotas de
// negócio. O estado fica só na memória do processo: um "kubectl rollout restart"
// sobe pods novos e limpa a falha, que é exatamente a ação corretiva automatizada.
// O /health continua respondendo 200, para que a liveness probe do Kubernetes
// não "conserte" a falha sozinha e o self-healing seja feito pela automação do alerta.
var chaosEnabled atomic.Bool

type chaosRequest struct {
	Enabled bool `json:"enabled"`
}

// chaosHandler liga/desliga a falha. Protegido pela MASTER_KEY (masterKeyAuthMiddleware).
func (a *App) chaosHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		return
	}

	var req chaosRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Corpo da requisição inválido", http.StatusBadRequest)
		return
	}

	chaosEnabled.Store(req.Enabled)
	if req.Enabled {
		log.Println("CHAOS: modo de falha ATIVADO - respostas de negócio retornarão HTTP 500")
	} else {
		log.Println("CHAOS: modo de falha desativado")
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]bool{"chaos_enabled": req.Enabled})
}

// chaosMiddleware devolve 500 quando a falha está ativa (exceto /health e o próprio /admin/chaos)
func chaosMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if chaosEnabled.Load() && r.URL.Path != "/health" && r.URL.Path != "/admin/chaos" {
			log.Printf("CHAOS: falha injetada em %s %s", r.Method, r.URL.Path)
			http.Error(w, `{"error": "falha injetada (chaos)"}`, http.StatusInternalServerError)
			return
		}
		next.ServeHTTP(w, r)
	})
}
