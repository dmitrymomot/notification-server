package main

import (
	"sort"
	"sync"
	"time"
)

const (
	defaultChannelLength int = 1024
)

// MemStorage struct
type MemStorage struct {
	sync.RWMutex
	events map[string][]Event
}

// NewMemStorage is a factory func, returns a new instance of the MemStorage structure
func NewMemStorage() *MemStorage {
	return &MemStorage{
		events: make(map[string][]Event, defaultChannelLength),
	}
}

// GetAllInChannel returns all events in channel
func (s *MemStorage) GetAllInChannel(channelID string) []Event {
	s.RLock()
	defer s.RUnlock()
	if events, ok := s.events[channelID]; ok {
		return events
	}
	return nil
}

// GetByLastID returns events in a channel which has id greater than given one
func (s *MemStorage) GetByLastID(channelID string, lastEventID int64) []Event {
	s.RLock()
	defer s.RUnlock()

	events, ok := s.events[channelID]
	if !ok {
		return nil
	}

	i := positionGt(events, lastEventID)
	if i >= 0 && i < len(events) {
		return events[i+1:]
	} else if i == -2 {
		return events
	}

	return nil
}

// Add event to storage
func (s *MemStorage) Add(channelID string, event Event) error {
	s.Lock()
	defer s.Unlock()

	events, ok := s.events[channelID]
	if !ok {
		events = make([]Event, 0, defaultChannelLength)
	}
	s.events[channelID] = sortEvents(append(events, event))

	return nil
}

// Delete event from storage
func (s *MemStorage) Delete(channelID string, event Event) error {
	s.Lock()
	defer s.Unlock()

	events, ok := s.events[channelID]
	if !ok {
		return nil
	}

	i := position(events, event.ID)
	if i >= 0 {
		events = sortEvents(append(events[:i], events[i+1:]...))
		s.events[channelID] = events
	}

	return nil
}

// GC - garbage collector
func (s *MemStorage) GC(eventMaxAge, gcPeriod string, wg *sync.WaitGroup) error {
	if eventMaxAge == "" {
		eventMaxAge = "1h"
	}
	maxAge, err := time.ParseDuration(eventMaxAge)
	if err != nil {
		return err
	}

	if gcPeriod == "" {
		gcPeriod = "1h"
	}
	period, err := time.ParseDuration(gcPeriod)
	if err != nil {
		return err
	}

	ticker := time.NewTicker(period)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			wg.Add(1)
			s.gc(maxAge)
			wg.Done()
		}
	}
}

func (s *MemStorage) gc(maxAge time.Duration) error {
	s.RLock()
	channels := make([]string, 0, len(s.events))
	for ch := range s.events {
		channels = append(channels, ch)
	}
	s.RUnlock()

	t := time.Now().UnixNano() - maxAge.Nanoseconds()

	for _, channelID := range channels {
		if err := s.deleteBefore(channelID, t); err != nil {
			return err
		}
	}

	return nil
}

// deleteBefore deletes event which is older then given time
func (s *MemStorage) deleteBefore(channelID string, t int64) error {
	s.Lock()
	defer s.Unlock()

	events, ok := s.events[channelID]
	if !ok {
		return nil
	}

	i := positionGt(events, t)
	if i >= 0 {
		l := len(events[i:])
		c := defaultChannelLength
		if l > c {
			c = l
		}
		truncated := make([]Event, l, c)
		copy(truncated, events[i:])
		s.events[channelID] = truncated
	}

	return nil
}

// Sort events by id
func sortEvents(events []Event) []Event {
	if len(events) > 1 {
		sort.SliceStable(events, func(i, j int) bool {
			return events[i].ID < events[j].ID
		})
	}
	return events
}

func position(a []Event, id int64) (res int) {
	mid := len(a) / 2
	switch {
	case len(a) == 0:
		res = -1
	case a[mid].ID > id:
		res = position(a[:mid], id)
	case a[mid].ID < id:
		res = position(a[mid+1:], id)
		if res >= 0 {
			res += mid + 1
		}
	default:
		res = mid
	}
	return res
}

func positionGt(a []Event, s int64) (res int) {
	mid := len(a) / 2
	switch {
	case len(a) == 0:
		res = -1
	case a[0].ID > s:
		res = 0
	case a[0].ID == s:
		res = 1
	case a[len(a)-1].ID <= s:
		res = len(a)
	case a[mid].ID < s && a[mid+1].ID > s:
		res = mid + 1
	case a[mid].ID > s:
		res = positionGt(a[:mid], s)
	case a[mid].ID < s:
		res = positionGt(a[mid+1:], s)
		if res >= 0 {
			res += mid + 1
		}
	default:
		res = mid
	}
	return res
}
