package main

import (
	"log"
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
	if i >= 0 {
		return events[i:]
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

// DeleteExpired deletes event which is older then given time from channel
func (s *MemStorage) DeleteExpired(channelID string, ttl string) error {
	d, err := time.ParseDuration(ttl)
	if err != nil {
		return err
	}
	t := time.Now().UnixNano() - d.Nanoseconds()
	log.Printf("parsed time: %d = %d - %d", t, time.Now().UnixNano(), d.Nanoseconds())

	s.Lock()
	defer s.Unlock()

	events, ok := s.events[channelID]
	if !ok {
		return nil
	}

	i := positionGt(events, t)
	log.Printf("position: %+v", i)
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
	case len(a) == 0 || a[0].ID > s:
		res = -1
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
		res = mid + 1
	}
	return res
}
