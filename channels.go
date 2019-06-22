package main

import "github.com/dustin/go-broadcast"

var channels = make(map[string]broadcast.Broadcaster)

func openListener(channelID string) chan interface{} {
	listener := make(chan interface{})
	channel(channelID).Register(listener)
	return listener
}

func closeListener(channelID string, listener chan interface{}) {
	channel(channelID).Unregister(listener)
	close(listener)
}

func openMultiChannelListener(channels []string) chan interface{} {
	listener := make(chan interface{})
	for _, channelID := range channels {
		channel(channelID).Register(listener)
	}
	return listener
}

func closeMultiChannelListener(channels []string, listener chan interface{}) {
	for _, channelID := range channels {
		channel(channelID).Unregister(listener)
	}
	close(listener)
}

func deleteBroadcast(channelID string) {
	b, ok := channels[channelID]
	if ok {
		b.Close()
		delete(channels, channelID)
	}
}

func channel(channelID string) broadcast.Broadcaster {
	b, ok := channels[channelID]
	if !ok {
		b = broadcast.NewBroadcaster(10)
		channels[channelID] = b
	}
	return b
}
