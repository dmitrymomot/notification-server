package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"github.com/gin-contrib/sse"
	"github.com/go-chi/chi"
	"github.com/go-chi/jwtauth"
	uuid "github.com/satori/go.uuid"
)

type (
	// Handler structure
	Handler struct {
		log Logger
		sse *SSE
	}

	// EventDataRequest struct
	EventDataRequest struct {
		Title   string      `json:"title"`
		Payload interface{} `json:"payload"`
		TTL     int64       `json:"ttl"`
	}
)

// NewHandler is a factory function, returns a new instance of the Handler structure
func NewHandler(log Logger, sse *SSE) *Handler {
	return &Handler{
		log: log,
		sse: sse,
	}
}

// Router returns instance of the chi.Router
func (h *Handler) Router() chi.Router {
	r := chi.NewRouter()

	r.Get("/", h.healthCheck)
	r.Get("/health", h.healthCheck)

	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	r.Route("/listen", func(r chi.Router) {
		if os.Getenv("BASIC_AUTH_USER") != "" && os.Getenv("BASIC_AUTH_PASSWORD") != "" {
			r.Use(basicAuth(os.Getenv("BASIC_AUTH_USER"), os.Getenv("BASIC_AUTH_PASSWORD")))
		}
		r.HandleFunc("/", h.listener)
		r.HandleFunc("/dump", h.dump)
		r.HandleFunc("/single/{channel}", h.subscribeToSingleChannel)
		r.HandleFunc("/multi/{channels}", h.subscribeToMultiChannels)
	})

	r.Route("/sub", func(r chi.Router) {
		if os.Getenv("JWT_SECRET") != "" {
			r.Use(jwtauth.Verify(jwtauth.New("HS256", []byte(os.Getenv("JWT_SECRET")), nil), tokenFromQuery))
			r.Use(jwtauth.Authenticator)
		}

		r.Get("/{channel}", h.subscribeToSingleChannel)
	})

	r.Route("/multisub-split", func(r chi.Router) {
		if os.Getenv("JWT_SECRET") != "" {
			r.Use(jwtauth.Verify(jwtauth.New("HS256", []byte(os.Getenv("JWT_SECRET")), nil), tokenFromQuery))
			r.Use(jwtauth.Authenticator)
		}

		r.Get("/{channels}", h.subscribeToMultiChannels)
	})

	r.Route("/pub", func(r chi.Router) {
		if os.Getenv("BASIC_TOKEN") != "" {
			r.Use(basicToken(os.Getenv("BASIC_TOKEN")))
		}

		r.Post("/{channel}", h.publishToChannel)
	})

	return r
}

func (h *Handler) healthCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte("."))
}

func (h *Handler) dump(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	channelID := r.FormValue("channel")
	if channelID == "" {
		channelID = chi.URLParam(r, "channel")
	}

	dump := prettyPrint(h.sse.DumpStorage(channelID))

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tmpl := template.Must(template.ParseFiles("/static/dump.html"))
	err := tmpl.Execute(w, map[string]interface{}{
		"channel": r.FormValue("channel"),
		"dump":    dump,
	})
	if err != nil {
		h.log.Errorf("parse template: %v", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}

func (h *Handler) listener(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	channelID := r.FormValue("channel")
	if channelID == "" {
		channelID = chi.URLParam(r, "channel")
	}

	stype := r.FormValue("type")
	if stype == "" {
		stype = "1"
	}
	endpoint := "/listen/single"
	if stype == "2" {
		endpoint = "/listen/multi"
	}

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tmpl := template.Must(template.ParseFiles("/static/index.html"))
	err := tmpl.Execute(w, map[string]interface{}{
		"channel":       r.FormValue("channel"),
		"last_event_id": r.FormValue("last_event_id"),
		"type":          stype,
		"endpoint":      endpoint,
	})
	if err != nil {
		h.log.Errorf("parse template: %v", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}

func (h *Handler) publishToChannel(w http.ResponseWriter, r *http.Request) {
	channelID := chi.URLParam(r, "channel")
	if channelID == "" {
		http.Error(w, "Missed channel id!", http.StatusBadRequest)
		return
	}
	payload := EventDataRequest{}
	if err := decodeJSON(r.Body, &payload); err != nil {
		h.log.Errorf("decode json: %v", err)
		http.Error(w, "Malformed JSON", http.StatusBadRequest)
		return
	}

	eventData := EventData{
		Title:   payload.Title,
		Payload: payload.Payload,
	}
	if err := h.sse.PubEvent(channelID, eventData, payload.TTL); err != nil {
		h.log.Errorf("publish to channel %s: %v", channelID, err)
		http.Error(w, fmt.Sprintf("could not publish to channel %s", channelID), http.StatusBadRequest)
		return
	}

	h.log.Debugf("[sent_event] publish to channel %s with payload: %+v", channelID, payload)

	w.Write([]byte("event has been sent"))
}

func (h *Handler) subscribeToSingleChannel(w http.ResponseWriter, r *http.Request) {
	channelID := chi.URLParam(r, "channel")
	if channelID == "" {
		h.log.Debugf("missed channel id")
		http.Error(w, "Missed channel id!", http.StatusBadRequest)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		h.log.Warnf("streaming unsupported: channel %s", channelID)
		http.Error(w, "Streaming unsupported!", http.StatusNotImplemented)
		return
	}

	listener, history, err := h.sse.SubscribeToChannel(channelID, getLastEventID(r))
	if err != nil {
		h.log.Errorf("subscribe to channel %s with last event id %s", channelID, getLastEventID(r))
		http.Error(w, "Could not subscribe to events channel", http.StatusInternalServerError)
		return
	}
	defer h.sse.Unsubscribe(channelID, listener)

	// Set the headers related to event streaming.
	if err := openHTTPConnection(w, r); err != nil {
		h.log.Errorf("open http connection: %s", err.Error())
		return
	}
	flusher.Flush()

	h.log.Debugf("[client_connected] client connected: %s", channelID)

	// send historical events
	for _, event := range history {
		if !event.IsExpired() {
			err := sse.Encode(w, event.MapToSseEvent())
			if err != nil {
				h.log.Errorf("sse encoding: %s (channel id: %s, event: %#v)", err.Error(), channelID, event)
				return
			}
			flusher.Flush()
			h.log.Debugf("[channel_received_event] channel %s: received event: %+v", channelID, event)
		}
	}

	for {
		select {
		case <-r.Context().Done():
			h.log.Debugf("[client_disconnected] client closed connection: %s", channelID)
			return
		case event := <-listener:
			if e, ok := event.(Event); ok {
				err := sse.Encode(w, e.MapToSseEvent())
				if err != nil {
					h.log.Errorf("sse encoding: %s (channel id: %s, event: %#v)", err.Error(), channelID, event)
					return
				}
				flusher.Flush()
				h.log.Debugf("[channel_received_event] channel %s: received event: %+v", channelID, event)
			} else {
				h.log.Errorf("event is not Event type: %#v", event)
			}
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

	listener, history, err := h.sse.SubscribeToMultiChannel(channels, getLastEventID(r))
	if err != nil {
		h.log.Errorf("subscribe to channels group %s with last event id %s", channelsStr, getLastEventID(r))
		http.Error(w, "Could not subscribe to events channel", http.StatusInternalServerError)
		return
	}
	defer h.sse.UnsubscribeFromMultiChannel(channels, listener)

	// Set the headers related to event streaming.
	if err := openHTTPConnection(w, r); err != nil {
		h.log.Errorf("open http connection: %s (channels: %s)", err.Error(), channelsStr)
		return
	}
	flusher.Flush()

	h.log.Debugf("[client_connected_group] client connected to channels group: %s", channelsStr)

	// send historical events
	for _, event := range history {
		if !event.IsExpired() {
			err := sse.Encode(w, event.MapToSseEvent())
			if err != nil {
				h.log.Errorf("sse encoding: %s (channels group: %s, event: %#v)", err.Error(), channelsStr, event)
				return
			}
			flusher.Flush()
			h.log.Debugf("[group_received_event] channels group %s received event: %+v", channelsStr, event)
		}
	}

	for {
		select {
		case <-r.Context().Done():
			h.log.Debugf("[client_disconnected_group] client closed connection to channels group: %s", channelsStr)
			return
		case event := <-listener:
			if e, ok := event.(Event); ok {
				err := sse.Encode(w, e.MapToSseEvent())
				if err != nil {
					h.log.Errorf("sse encoding: %s (channels: %s, event: %#v)", err.Error(), channelsStr, event)
					return
				}
				flusher.Flush()
				h.log.Debugf("[group_received_event] channels group %s received event: %+v", channelsStr, event)
			} else {
				h.log.Errorf("event is not sse.Event type: %#v", event)
			}
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
	lastEventID := r.Header.Get("Last-Event-ID")
	if lastEventID == "" {
		lastEventID = r.URL.Query().Get("last_event_id")
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
