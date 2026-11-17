package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"

	"github.com/XSAM/otelsql"
	_ "github.com/jackc/pgx/v4/stdlib"
	"github.com/joho/godotenv"
	"go.opentelemetry.io/otel/attribute"
)

// App struct (para injeção de dependência)
type App struct {
	DB        *sql.DB
	MasterKey string
}

func main() {
	_ = godotenv.Load()

	// Telemetria (OpenTelemetry): se falhar, o serviço continua funcionando sem ela
	shutdown, err := setupOTel(context.Background())
	if err != nil {
		log.Printf("Aviso: OpenTelemetry não inicializado: %v", err)
	} else {
		defer func() { _ = shutdown(context.Background()) }()
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8001" // Porta padrão
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL deve ser definida")
	}

	masterKey := os.Getenv("MASTER_KEY")
	if masterKey == "" {
		log.Fatal("MASTER_KEY deve ser definida")
	}

	db, err := connectDB(databaseURL)
	if err != nil {
		log.Fatalf("Não foi possível conectar ao banco de dados: %v", err)
	}
	defer db.Close()

	app := &App{
		DB:        db,
		MasterKey: masterKey,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", app.healthHandler)

	mux.HandleFunc("/validate", app.validateKeyHandler)

	mux.Handle("/admin/keys", app.masterKeyAuthMiddleware(http.HandlerFunc(app.createKeyHandler)))

	log.Printf("Serviço de Autenticação (Go) rodando na porta %s", port)
	if err := http.ListenAndServe(":"+port, instrumentHandler(mux, "auth-service")); err != nil {
		log.Fatal(err)
	}
}

// connectDB inicializa e testa a conexão com o PostgreSQL.
// O driver é envolvido pelo otelsql, que gera um span CLIENT para cada query.
func connectDB(databaseURL string) (*sql.DB, error) {
	db, err := otelsql.Open("pgx", databaseURL,
		otelsql.WithAttributes(attribute.String("db.system.name", "postgresql")),
	)
	if err != nil {
		return nil, err
	}

	if err = db.Ping(); err != nil {
		return nil, err
	}

	log.Println("Conectado ao PostgreSQL com sucesso!")
	return db, nil
}
