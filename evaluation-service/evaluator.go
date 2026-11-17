package main

import (
	"context"
	"crypto/sha1"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

var tracer = otel.Tracer("evaluation-service")

const (
	CACHE_TTL = 30 * time.Second
)

// getDecision é o wrapper principal
func (a *App) getDecision(ctx context.Context, userID, flagName string) (bool, error) {
	info, err := a.getCombinedFlagInfo(ctx, flagName)
	if err != nil {
		return false, err
	}

	return a.runEvaluationLogic(info, userID), nil
}

// getCombinedFlagInfo busca os dados no Redis, com fallback para os microsserviços
func (a *App) getCombinedFlagInfo(ctx context.Context, flagName string) (*CombinedFlagInfo, error) {
	cacheKey := fmt.Sprintf("flag_info:%s", flagName)

	val, err := a.redisGet(ctx, cacheKey)
	if err == nil {
		// Cache HIT
		var info CombinedFlagInfo
		if err := json.Unmarshal([]byte(val), &info); err == nil {
			log.Printf("Cache HIT para flag '%s'", flagName)
			return &info, nil
		}
		log.Printf("Erro ao desserializar cache para flag '%s': %v", flagName, err)
	}

	log.Printf("Cache MISS para flag '%s'", flagName)
	info, err := a.fetchFromServices(ctx, flagName)
	if err != nil {
		return nil, err
	}

	jsonData, err := json.Marshal(info)
	if err == nil {
		a.redisSet(ctx, cacheKey, jsonData)
	}

	return info, nil
}

// fetchFromServices busca dados do flag-service e targeting-service concorrentemente
func (a *App) fetchFromServices(ctx context.Context, flagName string) (*CombinedFlagInfo, error) {
	var wg sync.WaitGroup
	wg.Add(2)

	var flagInfo *Flag
	var ruleInfo *TargetingRule
	var flagErr, ruleErr error

	go func() {
		defer wg.Done()
		flagInfo, flagErr = a.fetchFlag(ctx, flagName)
	}()

	go func() {
		defer wg.Done()
		ruleInfo, ruleErr = a.fetchRule(ctx, flagName)
	}()

	wg.Wait()

	if flagErr != nil {
		return nil, flagErr
	}
	if ruleErr != nil {
		log.Printf("Aviso: Nenhuma regra de segmentação encontrada para '%s'. Usando padrão.", flagName)
	}

	return &CombinedFlagInfo{
		Flag: flagInfo,
		Rule: ruleInfo,
	}, nil
}

// fetchFlag (função helper)
func (a *App) fetchFlag(ctx context.Context, flagName string) (*Flag, error) {
	url := fmt.Sprintf("%s/flags/%s", a.FlagServiceURL, url.PathEscape(flagName))

	apiKey := os.Getenv("SERVICE_API_KEY")
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil) // #nosec G704 -- URL base vem de variável de ambiente confiável; flagName é sanitizado via url.PathEscape acima
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := a.HttpClient.Do(req) // #nosec G704 -- URL sanitizada, ver comentário acima
	if err != nil {
		return nil, fmt.Errorf("erro ao chamar flag-service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, &NotFoundError{flagName}
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("flag-service retornou status %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var flag Flag
	if err := json.Unmarshal(body, &flag); err != nil {
		return nil, fmt.Errorf("erro ao desserializar resposta do flag-service: %w", err)
	}
	return &flag, nil
}

func (a *App) fetchRule(ctx context.Context, flagName string) (*TargetingRule, error) {
	url := fmt.Sprintf("%s/rules/%s", a.TargetingServiceURL, url.PathEscape(flagName))
	apiKey := os.Getenv("SERVICE_API_KEY")                              // Usa a mesma chave
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil) // #nosec G704 -- URL base vem de variável de ambiente confiável; flagName é sanitizado via url.PathEscape acima
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := a.HttpClient.Do(req) // #nosec G704 -- URL sanitizada, ver comentário acima
	if err != nil {
		return nil, fmt.Errorf("erro ao chamar targeting-service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, &NotFoundError{flagName} // Não é um erro fatal
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("targeting-service retornou status %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var rule TargetingRule
	if err := json.Unmarshal(body, &rule); err != nil {
		return nil, fmt.Errorf("erro ao desserializar resposta do targeting-service: %w", err)
	}
	return &rule, nil
}

// runEvaluationLogic é onde a decisão é tomada
func (a *App) runEvaluationLogic(info *CombinedFlagInfo, userID string) bool {
	if info.Flag == nil || !info.Flag.IsEnabled {
		return false
	}

	if info.Rule == nil || !info.Rule.IsEnabled {
		return true
	}

	rule := info.Rule.Rules
	if rule.Type == "PERCENTAGE" {
		percentage, ok := rule.Value.(float64)
		if !ok {
			log.Printf("Erro: valor da regra de porcentagem não é um número para a flag '%s'", info.Flag.Name)
			return false
		}

		userBucket := getDeterministicBucket(userID + info.Flag.Name)

		if float64(userBucket) < percentage {
			return true
		}
	}

	return false
}

func getDeterministicBucket(input string) int {
	hasher := sha1.New()
	hasher.Write([]byte(input))
	hash := hasher.Sum(nil)

	val := binary.BigEndian.Uint32(hash[:4])

	return int(val % 100)
}

// redisGet lê do cache com um span CLIENT, para o Redis aparecer no trace e no Service Map
func (a *App) redisGet(ctx context.Context, key string) (string, error) {
	ctx, span := a.startRedisSpan(ctx, "GET")
	defer span.End()

	val, err := a.RedisClient.Get(ctx, key).Result()
	span.SetAttributes(attribute.Bool("cache.hit", err == nil))
	return val, err
}

// redisSet grava no cache com TTL, também instrumentado
func (a *App) redisSet(ctx context.Context, key string, value []byte) {
	ctx, span := a.startRedisSpan(ctx, "SET")
	defer span.End()

	if err := a.RedisClient.Set(ctx, key, value, CACHE_TTL).Err(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
}

func (a *App) startRedisSpan(ctx context.Context, operation string) (context.Context, trace.Span) {
	return tracer.Start(ctx, "redis "+operation,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("db.system.name", "redis"),
			attribute.String("db.operation.name", operation),
			attribute.String("server.address", a.RedisClient.Options().Addr),
		),
	)
}
