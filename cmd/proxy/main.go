package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"iron/internal/embedding"
	"iron/internal/nlu"
	"iron/internal/proxy"
	"iron/middlewares/contextcompressor"
	"iron/middlewares/rag"
	"iron/middlewares/semanticcache"
	"iron/middlewares/websearch"
)

func main() {
	configPath := flag.String("config", "", "path to config file")
	flag.Parse()

	cfg, err := proxy.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	srv := proxy.NewServer(cfg)

	embedClient := embedding.NewClient(cfg.Ollama.BaseURL, cfg.Ollama.EmbeddingModel)
	nluRouter := nlu.NewRouter(cfg.Ollama.BaseURL, cfg.Ollama.FastModel)

	middlewares := []proxy.Middleware{
		semanticcache.New("http://localhost:8000", embedClient, cfg.Cache.SimilarityThreshold, cfg.Cache.TTLHours),
		rag.New("http://localhost:8000", cfg.RAG.ChromaPath, cfg.RAG.DefaultCollection, cfg.RAG.TopK, embedClient, nluRouter),
		websearch.New(nluRouter, cfg.Search.MaxResults, cfg.Search.TimeoutSeconds),
		contextcompressor.New(cfg.Ollama.BaseURL, cfg.Ollama.CompressionModel, cfg.Ollama.CompressionThreshold),
	}
	srv.SetupPipelineWith(middlewares...)

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	addr := fmt.Sprintf("%s:%d", cfg.Proxy.Host, cfg.Proxy.Port)
	httpSrv := &http.Server{Addr: addr, Handler: mux}

	go func() {
		log.Printf("proxy listening on %s", addr)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("proxy server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	httpSrv.Shutdown(ctx)
}
