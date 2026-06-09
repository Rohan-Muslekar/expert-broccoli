package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"cheat-detection/game-server/config"
	"cheat-detection/game-server/server"
	"cheat-detection/game-server/telemetry"
)

func main() {
	cfg := config.Load()
	log.Printf("starting game server on :%d (tick_rate=%d, bots=%d, kafka=%v)",
		cfg.Port, cfg.TickRate, cfg.BotCount, cfg.KafkaEnabled)

	game := server.NewGame(cfg)
	srv := server.NewServer(game)
	producer := telemetry.NewProducer(cfg.KafkaEnabled, cfg.KafkaBrokers)
	defer producer.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		for t := range game.TelemetryCh() {
			producer.PublishTelemetry(ctx, t)
		}
	}()
	go func() {
		for k := range game.KillsCh() {
			producer.PublishKill(ctx, k)
		}
	}()

	tickInterval := time.Duration(1000/cfg.TickRate) * time.Millisecond
	go func() {
		ticker := time.NewTicker(tickInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				game.RunTick(time.Now())
				srv.BroadcastState()
			}
		}
	}()

	http.HandleFunc("/ws", srv.HandleWS)
	http.HandleFunc("/dashboard-ws", srv.HandleDashboardWS)

	addr := fmt.Sprintf(":%d", cfg.Port)
	httpSrv := &http.Server{Addr: addr}

	go func() {
		if err := httpSrv.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("http server error: %v", err)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	log.Println("shutting down...")
	cancel()
	httpSrv.Shutdown(context.Background())
}
