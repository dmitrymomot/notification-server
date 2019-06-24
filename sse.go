package main

import (
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/gin-contrib/sse"
)

type (
	// Event struct
	Event struct {
		ID        int64
		Data      EventData
		TTL       int64 `json:"-"`
		Timestamp int64 `json:"-"`
	}

	// EventData struct
	EventData struct {
		Title   string      `json:"title"`
		Payload interface{} `json:"payload"`
	}

	// SSE struct
	SSE struct {
		storage Storage
	}
)

// MapToSseEvent func to convert struct Event to sse.Event
func (e Event) MapToSseEvent() sse.Event {
	t := time.Unix(0, e.ID)
	id := fmt.Sprintf("%d:%d", t.Unix(), t.Nanosecond())
	return sse.Event{
		Id:    id,
		Event: "message",
		Data:  e.Data,
	}
}

// IsExpired func
func (e Event) IsExpired() bool {
	return time.Now().After(time.Unix(0, e.Timestamp).Add(time.Duration(e.TTL) * time.Second))
}

// NewSSE factory
func NewSSE(storage Storage) *SSE {
	return &SSE{storage: storage}
}

// PubEvent func publishes data to channel
func (s *SSE) PubEvent(channelID string, data EventData, ttl int64) error {
	t := time.Now().UnixNano()
	event := Event{
		ID:        t,
		Data:      data,
		TTL:       ttl,
		Timestamp: t,
	}
	channel(channelID).Submit(event)
	return s.storeEvent(channelID, event)
}

// SubscribeToChannel func
func (s *SSE) SubscribeToChannel(channelID, lastEventID string) (chan interface{}, []Event, error) {
	listener := openListener(channelID)
	history := make([]Event, 0, 50)
	var err error
	if lastEventID != "" {
		history, err = s.getEventsByLastID(channelID, lastEventID)
		if err != nil {
			return nil, nil, err
		}
	}
	return listener, history, err
}

// SubscribeToMultiChannel func
func (s *SSE) SubscribeToMultiChannel(channels []string, lastEventID string) (chan interface{}, []Event, error) {
	listener := openMultiChannelListener(channels)
	history := make([]Event, 0, 100)
	if lastEventID != "" {
		for _, channelID := range channels {
			events, err := s.getEventsByLastID(channelID, lastEventID)
			if err != nil {
				return nil, nil, err
			}
			log.Printf("channel: %s;\nlast id: %s;\nevents: %+v\n\n", channelID, lastEventID, events)
			history = append(history, events...)
			log.Printf("history: %+v", history)
		}
	}
	return listener, history, nil
}

// Unsubscribe from channel
func (s *SSE) Unsubscribe(channelID string, listener chan interface{}) error {
	closeListener(channelID, listener)
	return nil
}

// UnsubscribeFromMultiChannel from channel
func (s *SSE) UnsubscribeFromMultiChannel(channels []string, listener chan interface{}) error {
	closeMultiListener(channels, listener)
	return nil
}

// DumpStorage func
func (s *SSE) DumpStorage(channelID string) []Event {
	channelID = strings.ToLower(channelID)
	return s.storage.GetAllInChannel(channelID)
}

func (s *SSE) storeEvent(channelID string, event Event) error {
	channelID = strings.ToLower(channelID)
	return s.storage.Add(channelID, event)
}

func (s *SSE) getEventsByLastID(channelID, lastEventID string) ([]Event, error) {
	channelID = strings.ToLower(channelID)
	var events []Event
	if lastEventID != "" && strings.Contains(lastEventID, ":") {
		parts := strings.Split(lastEventID, ":")
		if len(parts) != 2 {
			return nil, errors.New("wrong last event id")
		}
		sec, err := strconv.Atoi(parts[0])
		if err != nil {
			return nil, fmt.Errorf("convert last event id to int64: %v", err)
		}
		nsec, err := strconv.Atoi(parts[1])
		if err != nil {
			return nil, fmt.Errorf("convert last event id to int64: %v", err)
		}
		lid := time.Unix(int64(sec), int64(nsec)).UnixNano()
		events = s.storage.GetByLastID(channelID, lid)
	}
	return events, nil
}
