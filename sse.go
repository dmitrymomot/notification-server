package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

type (
	// Event struct
	Event struct {
		ID   int64
		Data EventData
	}

	// EventData struct
	EventData struct {
		Title   string      `json:"title"`
		Payload interface{} `json:"payload"`
	}
)

// pubEvent func publishes data to channel
func pubEvent(channelID string, data EventData) error {
	event := Event{
		ID:   time.Now().UnixNano(),
		Data: data,
	}
	channel(channelID).Submit(data)
	return storeEvent(event)
}

func subscribeToChannel(channelID, lastEventID string) (chan interface{}, error) {
	listener := openListener(channelID)
	if lastEventID != "" {
		events, err := getEventsByLastID(channelID, lastEventID)
		if err != nil {
			return nil, err
		}
		for _, event := range events {
			listener <- event
		}
	}
	return listener, nil
}

func storeEvent(event Event) error {

	return nil
}

func getEventsByLastID(channelID, lastEventID string) ([]Event, error) {
	var events []Event
	if lastEventID != "" {
		lastEventID = strings.ReplaceAll(lastEventID, ":", "")
		lsid, err := strconv.Atoi(lastEventID)
		if err != nil {
			return nil, fmt.Errorf("convert last event id to int64: %v", err)
		}
		events = storageInstance.GetByLastID(channelID, int64(lsid))
	}
	return events, nil
}
