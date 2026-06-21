package feature

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"cheat-detection/game-server/metrics"
	"cheat-detection/game-server/telemetry"
	"github.com/segmentio/kafka-go"
)

type Engine struct {
	featureWriter *kafka.Writer
	alertWriter   *kafka.Writer
	processors    map[string]*playerSlot
	mu            sync.Mutex
	playerTimeout time.Duration
}

type playerSlot struct {
	processor *PlayerProcessor
	lastSeen  time.Time
}

func NewEngine(brokers []string, produceTopic, alertsTopic string, playerTimeoutSec int) *Engine {
	return &Engine{
		featureWriter: &kafka.Writer{
			Addr:         kafka.TCP(brokers...),
			Topic:        produceTopic,
			Balancer:     &kafka.LeastBytes{},
			BatchTimeout: 5 * time.Millisecond,
			Async:        true,
		},
		alertWriter: &kafka.Writer{
			Addr:         kafka.TCP(brokers...),
			Topic:        alertsTopic,
			Balancer:     &kafka.LeastBytes{},
			BatchTimeout: 5 * time.Millisecond,
			Async:        true,
		},
		processors:    make(map[string]*playerSlot),
		playerTimeout: time.Duration(playerTimeoutSec) * time.Second,
	}
}

func (engine *Engine) Run(ctx context.Context, events <-chan telemetry.PlayerTelemetry) {
	go engine.cleanupLoop(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-events:
			if !ok {
				return
			}
			start := time.Now()
			engine.processEvent(ctx, event)
			metrics.FeatureProcessingDuration.Observe(time.Since(start).Seconds())
		}
	}
}

func (engine *Engine) processEvent(ctx context.Context, event telemetry.PlayerTelemetry) {
	engine.mu.Lock()
	slot, exists := engine.processors[event.PlayerID]
	if !exists {
		slot = &playerSlot{
			processor: NewPlayerProcessor(event.PlayerID),
		}
		engine.processors[event.PlayerID] = slot
		metrics.FeatureActivePlayers.Inc()
	}
	slot.lastSeen = time.Now()
	engine.mu.Unlock()

	features := slot.processor.Process(event)

	featureData, err := json.Marshal(features)
	if err != nil {
		log.Printf("marshal feature error: %v", err)
		return
	}
	err = engine.featureWriter.WriteMessages(ctx, kafka.Message{
		Key:   []byte(features.PlayerID),
		Value: featureData,
	})
	if err != nil {
		log.Printf("feature publish error: %v", err)
	} else {
		metrics.FeaturesPublished.Inc()
	}

	alerts := Evaluate(features)
	for _, alert := range alerts {
		alertData, err := json.Marshal(alert)
		if err != nil {
			log.Printf("marshal alert error: %v", err)
			continue
		}
		err = engine.alertWriter.WriteMessages(ctx, kafka.Message{
			Key:   []byte(alert.PlayerID),
			Value: alertData,
		})
		if err != nil {
			log.Printf("alert publish error: %v", err)
		} else {
			metrics.FeatureAlertsFired.WithLabelValues(alert.Rule).Inc()
		}
	}
}

func (engine *Engine) cleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			engine.mu.Lock()
			now := time.Now()
			for playerID, slot := range engine.processors {
				if now.Sub(slot.lastSeen) > engine.playerTimeout {
					delete(engine.processors, playerID)
					metrics.FeatureActivePlayers.Dec()
					log.Printf("cleaned up feature processor: %s", playerID)
				}
			}
			engine.mu.Unlock()
		}
	}
}

func (engine *Engine) Close() {
	if engine.featureWriter != nil {
		engine.featureWriter.Close()
	}
	if engine.alertWriter != nil {
		engine.alertWriter.Close()
	}
}
