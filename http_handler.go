package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-contrib/sse"
	"github.com/go-chi/chi"
	"github.com/go-chi/jwtauth"
	"github.com/nats-io/stan.go"
	"github.com/nats-io/stan.go/pb"
	uuid "github.com/satori/go.uuid"
)

// Handler structure
type Handler struct {
	log Logger
}

// NewHandler is a factory function, returns a new instance of the Handler structure
func NewHandler(log Logger) *Handler {
	return &Handler{log: log}
}

// Router returns instance of the chi.Router
func (h *Handler) Router() chi.Router {
	r := chi.NewRouter()

	r.Get("/", h.healthCheck)
	r.Get("/health", h.healthCheck)

	r.Route("/sub", func(r chi.Router) {
		r.Use(jwtauth.Verify(jwtauth.New("HS256", []byte(os.Getenv("JWT_SECRET")), nil), tokenFromQuery))
		r.Use(jwtauth.Authenticator)

		r.Get("/user_notifications_{userID:[0-9]+}", h.subscribeToUserChannel)
	})

	r.Route("/multisub-split", func(r chi.Router) {
		r.Use(jwtauth.Verify(jwtauth.New("HS256", []byte(os.Getenv("JWT_SECRET")), nil), tokenFromQuery))
		r.Use(jwtauth.Authenticator)

		r.Get("/{channels}", h.subscribeToMultiChannels)
	})

	r.Route("/pub", func(r chi.Router) {
		r.Use(basicToken)

		r.Post("/{channelID}", h.publishToChannel)
	})

	return r
}

func (h *Handler) healthCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte("."))
}

func (h *Handler) publishToChannel(w http.ResponseWriter, r *http.Request) {
	channelID := chi.URLParam(r, "channelID")
	if channelID == "" {
		http.Error(w, "Missed channel id!", http.StatusBadRequest)
		return
	}
	payload := EventData{}
	if err := decodeJSON(r.Body, &payload); err != nil {
		h.log.Errorf("decode json: %v", err)
		http.Error(w, "Malformed JSON", http.StatusBadRequest)
		return
	}
	h.log.Debugf("publish to channel %s with payload: %+v", channelID, payload)

	if err := pubEvent(channelID, payload); err != nil {
		h.log.Errorf("publish to channel %s: %v", channelID, err)
		http.Error(w, fmt.Sprintf("could not publish to channel %s", channelID), http.StatusBadRequest)
		return
	}

	w.Write([]byte("event has been sent"))
}

func (h *Handler) subscribeToUserChannel(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userID")
	if userID == "" {
		h.log.Debugf("missed user id")
		http.Error(w, "Missed user id!", http.StatusBadRequest)
		return
	}
	channelID := fmt.Sprintf("user_notifications_%s", userID)

	flusher, ok := w.(http.Flusher)
	if !ok {
		h.log.Warnf("streaming unsupported: user_id: %s", userID)
		http.Error(w, "Streaming unsupported!", http.StatusNotImplemented)
		return
	}

	listener := openListener(channelID)
	defer closeListener(channelID, listener)

	// Set the headers related to event streaming.
	if err := openHTTPConnection(w, r); err != nil {
		h.log.Errorf("open http connection: %s", err.Error())
		return
	}
	flusher.Flush()

	h.log.Debugf("client connected: %s", userID)

	for {
		select {
		case <-r.Context().Done():
			h.log.Debugf("client disconnected: %s", userID)
			return
		case event := <-listener:
			if e, ok := event.(sse.Event); ok {
				h.log.Debugf("user %s received event: %+v", userID, event)
				err := sse.Encode(w, e)
				if err != nil {
					h.log.Errorf("sse encoding: %s (user id: %s, event: %#v)", err.Error(), userID, event)
					return
				}
				flusher.Flush()
			} else {
				h.log.Errorf("event is not sse.Event type: %#v", event)
			}
		default:
			flusher.Flush()
		}
	}
}

//multisub-split
func (h *Handler) subscribeToMultiChannels(w http.ResponseWriter, r *http.Request) {
	channelsStr := chi.URLParam(r, "channels")
	if channelsStr == "" {
		h.log.Debugf("missed channel id")
		http.Error(w, "Missed channel id!", http.StatusBadRequest)
		return
	}
	channels := strings.Split(channelsStr, ",")

	flusher, ok := w.(http.Flusher)
	if !ok {
		h.log.Warnf("streaming unsupported: channels: %s", channelsStr)
		http.Error(w, "Streaming unsupported!", http.StatusNotImplemented)
		return
	}

	listener := openMultiChannelListener(channels)
	defer closeMultiChannelListener(channels, listener)

	// Set the headers related to event streaming.
	if err := openHTTPConnection(w, r); err != nil {
		h.log.Errorf("open http connection: %s (channels: %s)", err.Error(), channelsStr)
		return
	}

	flusher.Flush()

	h.log.Debugf("client connected to channels group: %s", channelsStr)

	for {
		select {
		case <-r.Context().Done():
			h.log.Debugf("client disconnected from channels group: %s", channelsStr)
			return
		case event := <-listener:
			h.log.Debugf("channels group %s received event: %+v", channelsStr, event)
			if e, ok := event.(sse.Event); ok {
				err := sse.Encode(w, e)
				if err != nil {
					h.log.Errorf("sse encoding: %s (channels: %s, event: %#v)", err.Error(), channelsStr, event)
					return
				}
				flusher.Flush()
			} else {
				h.log.Errorf("event is not sse.Event type: %#v", event)
			}
		default:
			flusher.Flush()
		}
	}
}

func clientID(r *http.Request) string {
	clientID := r.URL.Query().Get("token")
	if clientID == "" {
		clientID = uuid.NewV1().String()
	}
	return clientID
}

func getLastEventID(r *http.Request) string {
	lastEventID := r.URL.Query().Get("last_event_id")
	if lastEventID == "" {
		lastEventID = r.Header.Get("Last-Event-ID")
	}
	return lastEventID
}

func decodeJSON(r io.Reader, v interface{}) error {
	defer io.Copy(ioutil.Discard, r)
	return json.NewDecoder(r).Decode(v)
}

// Set the headers related to event streaming.
func openHTTPConnection(w http.ResponseWriter, r *http.Request) error {
	origin := r.Header.Get("Origin")
	if origin == "" {
		w.Header().Set("Access-Control-Allow-Origin", "*")
	} else {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Credentials", "true")
	}
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Transfer-Encoding", "chunked")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	return sse.Encode(w, sse.Event{
		Event: "notification",
		Data:  "SSE connection successfully established",
	})
}

func startOption(lastEventID string) (stan.SubscriptionOption, error) {
	startOpt := stan.StartAt(pb.StartPosition_NewOnly)
	if lastEventID != "" {
		var startTime time.Time
		var sequence int
		opts := strings.Split(lastEventID, ":")
		if len(opts) == 2 {
			t := opts[0]
			t2, err := strconv.ParseInt(t, 10, 64)
			if err != nil {
				log.Printf("[error] parse last event id %s: %+v", lastEventID, err)
				return nil, err
			}
			startTime = time.Unix(0, t2)
			sequence, err = strconv.Atoi(opts[1])
			if err != nil {
				log.Printf("[error] parse sequence %s: %+v", lastEventID, err)
				return nil, err
			}
			if sequence > 0 {
				startOpt = stan.StartAtSequence(uint64(sequence))
			} else {
				startOpt = stan.StartAtTime(startTime)
			}
		}
	}
	return startOpt, nil
}
