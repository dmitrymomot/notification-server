package main

type (
	// Storage interface
	Storage interface {
		// get all events in channel
		GetAllInChannel(channelID string) []Event
		// get events in a channel which has id greater than given one
		GetByLastID(channelID string, lastEventID int64) []Event
		// Add event to storage
		Add(channelID string, event Event) error
		// Delete event from storage
		Delete(channelID string, event Event) error
		// Deletes event which is older then given time from channel
		DeleteExpired(channelID string, ttl string) error
	}
)
