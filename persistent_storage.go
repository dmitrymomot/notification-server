package main

import (
	"fmt"
	"sync"
	"time"

	"github.com/dmitrymomot/tiedot/db"
	"github.com/dmitrymomot/tiedot/dberr"
	"github.com/mitchellh/mapstructure"
)

type (
	// PersistentStorage struct
	PersistentStorage struct {
		db  *db.DB
		log Logger
	}
)

// NewPersistentStorage is a factory func, returns a new instance of the PersistentStorage structure
func NewPersistentStorage(storageDir string, log Logger) (*PersistentStorage, error) {
	dbInst, err := db.OpenDB(storageDir)
	if err != nil {
		return nil, err
	}
	return &PersistentStorage{db: dbInst, log: log}, nil
}

// GetAllInChannel - get all events in channel
func (s *PersistentStorage) GetAllInChannel(channelID string) []Event {
	c, err := s.getCollection(channelID)
	if err != nil {
		s.log.Errorf("get collection %s: %+v", channelID, err)
		return nil
	}

	res := make(map[int]struct{}, 0)
	s.log.Debugf("collection: %+v, result: %+v", c, res)
	if err := db.EvalQuery("all", c, &res); err != nil {
		s.log.Errorf("get all events in channel %s: %+v", channelID, err)
		return nil
	}

	events := make([]Event, 0, len(res))
	for id := range res {
		doc, err := c.Read(id)
		if err != nil {
			s.log.Errorf("read document id %s from channel %s: %+v", id, channelID, err)
			return events
		}
		e := Event{}
		if err := mapstructure.Decode(doc, &e); err != nil {
			s.log.Errorf("decode document id %s to event for channel %s: %+v", id, channelID, err)
			return events
		}
		events = append(events, e)
	}

	return events
}

// GetByLastID - get events in a channel which has id greater than given one
func (s *PersistentStorage) GetByLastID(channelID string, lastEventID int64) []Event {
	c, err := s.getCollection(channelID)
	if err != nil {
		s.log.Errorf("get collection %s: %+v", channelID, err)
		return nil
	}

	query := map[string]interface{}{
		"int-from": lastEventID,
		"int-to":   time.Now().Add(time.Hour).UnixNano(),
		"in":       []interface{}{"id"},
	}

	s.log.Debugf("query: %+v", query)

	res := make(map[int]struct{}, 0)
	if err := db.EvalQuery(query, c, &res); err != nil {
		s.log.Errorf("get events by last id %d from channel %s: %+v", lastEventID, channelID, err)
		return nil
	}
	s.log.Debugf("result: %+v", res)

	events := make([]Event, 0, len(res))
	for id := range res {
		doc, err := c.Read(id)
		if err != nil {
			s.log.Errorf("read document id %s from channel %s: %+v", id, channelID, err)
			return events
		}
		s.log.Debugf("document: %+v", doc)
		e := Event{}
		if err := mapstructure.Decode(doc, &e); err != nil {
			s.log.Errorf("decode document id %s to event for channel %s: %+v", id, channelID, err)
			return events
		}
		s.log.Debugf("event: %+v", e)
		events = append(events, e)
	}

	s.log.Debugf("events: %+v", events)
	return events
}

// Add event to storage
func (s *PersistentStorage) Add(channelID string, event Event) error {
	c, err := s.getCollection(channelID)
	if err != nil {
		s.log.Errorf("get collection %s: %+v", channelID, err)
		return nil
	}

	query := make(map[string]interface{})
	if err := mapstructure.Decode(&event, &query); err != nil {
		return err
	}
	_, err = c.Insert(query)

	return err
}

// Delete event from storage
func (s *PersistentStorage) Delete(channelID string, event Event) error {
	c, err := s.getCollection(channelID)
	if err != nil {
		s.log.Errorf("get collection %s: %+v", channelID, err)
		return nil
	}

	query := map[string]interface{}{
		"eq": event.ID,
		"in": []interface{}{"id"},
	}

	res := make(map[int]struct{}, 0)
	if err := db.EvalQuery(query, c, &res); err != nil {
		s.log.Errorf("delete event by id %d from channel %s: %+v", event.ID, channelID, err)
		return nil
	}

	for id := range res {
		if err := c.Delete(id); err != nil && dberr.Type(err) != dberr.ErrorNoDoc {
			return err
		}
	}

	return nil
}

// Close storage
func (s *PersistentStorage) Close() error {
	return s.db.Close()
}

// GC - Deletes event which is older then given time from channel
func (s *PersistentStorage) GC(eventMaxAge, gcPeriod string, wg *sync.WaitGroup) error {
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

func (s *PersistentStorage) gc(maxAge time.Duration) error {
	t := time.Now().UnixNano() - maxAge.Nanoseconds()
	for _, name := range s.db.AllCols() {
		if c := s.db.Use(name); c != nil {
			c.ForEachDoc(func(id int, docContent []byte) (willMoveOn bool) {
				if t > int64(id) {
					if err := c.Delete(id); err != nil && dberr.Type(err) != dberr.ErrorNoDoc {
						s.log.Errorf("[gc] %d: %+v", id, err)
					}
				}
				return true
			})
		}
	}
	return nil
}

func (s *PersistentStorage) getCollection(name string) (*db.Col, error) {
	c := s.db.Use(name)
	if c == nil {
		if err := s.db.Create(name); err != nil {
			return nil, err
		}
		c = s.db.Use(name)
		if c == nil {
			return nil, fmt.Errorf("could not create collection: %s", name)
		}
		if err := c.Index([]string{"id"}); err != nil {
			return nil, err
		}
	}
	return c, nil
}
