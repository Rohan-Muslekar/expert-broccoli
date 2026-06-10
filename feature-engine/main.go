package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"cheat-detection/feature-engine/config"
	"cheat-detection/feature-engine/engine"
	"cheat-detection/feature-engine/rules"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	cfg := config.Load()
	log.Printf("starting feature engine (brokers=%v, consume=%s, produce=%s, alerts=%s)",
		cfg.KafkaBrokers, cfg.ConsumeTopic, cfg.ProduceTopic, cfg.AlertsTopic)

	pipeline := engine.NewPipeline(
		cfg.KafkaBrokers,
		cfg.ConsumeTopic,
		cfg.ConsumerGroup,
		cfg.ProduceTopic,
		cfg.AlertsTopic,
		cfg.PlayerTimeoutSec,
		rules.Evaluate,
	)
	defer pipeline.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		http.Handle("/metrics", promhttp.Handler())
		addr := fmt.Sprintf(":%d", cfg.MetricsPort)
		log.Printf("metrics server on %s", addr)
		if err := http.ListenAndServe(addr, nil); err != nil {
			log.Printf("metrics server error: %v", err)
		}
	}()

	go pipeline.Run(ctx)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	log.Println("shutting down...")
	cancel()
}
