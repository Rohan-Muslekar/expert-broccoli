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
	proc     *PlayerProcessor
	lastSeen time.Time
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

func (e *Engine) Run(ctx context.Context, events <-chan telemetry.PlayerTelemetry) {
	go e.cleanupLoop(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-events:
			if !ok {
				return
			}
			start := time.Now()
			e.processEvent(ctx, ev)
			metrics.FeatureProcessingDuration.Observe(time.Since(start).Seconds())
		}
	}
}

func (e *Engine) processEvent(ctx context.Context, ev telemetry.PlayerTelemetry) {
	e.mu.Lock()
	slot, exists := e.processors[ev.PlayerID]
	if !exists {
		slot = &playerSlot{
			proc: NewPlayerProcessor(ev.PlayerID),
		}
		e.processors[ev.PlayerID] = slot
		metrics.FeatureActivePlayers.Inc()
	}
	slot.lastSeen = time.Now()
	e.mu.Unlock()

	fv := slot.proc.Process(ev)

	fvData, err := json.Marshal(fv)
	if err != nil {
		log.Printf("marshal feature error: %v", err)
		return
	}
	err = e.featureWriter.WriteMessages(ctx, kafka.Message{
		Key:   []byte(fv.PlayerID),
		Value: fvData,
	})
	if err != nil {
		log.Printf("feature publish error: %v", err)
	} else {
		metrics.FeaturesPublished.Inc()
	}

	alerts := Evaluate(fv)
	for _, alert := range alerts {
		alertData, err := json.Marshal(alert)
		if err != nil {
			log.Printf("marshal alert error: %v", err)
			continue
		}
		err = e.alertWriter.WriteMessages(ctx, kafka.Message{
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

func (e *Engine) cleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			e.mu.Lock()
			now := time.Now()
			for id, slot := range e.processors {
				if now.Sub(slot.lastSeen) > e.playerTimeout {
					delete(e.processors, id)
					metrics.FeatureActivePlayers.Dec()
					log.Printf("cleaned up feature processor: %s", id)
				}
			}
			e.mu.Unlock()
		}
	}
}

func (e *Engine) Close() {
	if e.featureWriter != nil {
		e.featureWriter.Close()
	}
	if e.alertWriter != nil {
		e.alertWriter.Close()
	}
}
