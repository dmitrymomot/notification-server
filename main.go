package main

import (
	"encoding/json"
	"log"
	"time"

	uuid "github.com/satori/go.uuid"
)

var storageInstance Storage

func main() {
	storageInstance = NewMemStorage()
	channelID := "test_channel"
	lastID := time.Now().UnixNano()
	for i := 0; i < 50000; i++ {
		e := Event{
			ID: time.Now().UnixNano(),
			Data: EventData{
				Title: uuid.NewV1().String(),
				Payload: map[string]interface{}{
					"order_id": i,
					"type":     "new_order",
				},
			},
		}
		storageInstance.Add(channelID, e)
		if i == 49997 {
			lastID = e.ID
		}
		if i > 49995 {
			log.Printf("added event %d", e.ID)
		}
		// time.Sleep(200 * time.Millisecond)
	}
	all := len(storageInstance.GetAllInChannel(channelID))
	// log.Printf("all events: %s", prettyPrint(storageInstance.GetAllInChannel(channelID)))
	log.Printf("last_id: %d", lastID)
	log.Printf("get events by last id %d: %s", lastID, prettyPrint(storageInstance.GetByLastID(channelID, lastID)))
	storageInstance.DeleteExpired(channelID, "1s")
	tr := len(storageInstance.GetAllInChannel(channelID))
	// log.Printf("all events after clean expired: %s", prettyPrint(storageInstance.GetAllInChannel(channelID)))
	// log.Printf("get events by last id %d: %s", lastID, prettyPrint(storageInstance.GetByLastID(channelID, lastID)))
	log.Printf("all: %d, after trim: %d", all, tr)
}

func prettyPrint(i interface{}) string {
	s, _ := json.MarshalIndent(i, "", "\t")
	return string(s)
}
