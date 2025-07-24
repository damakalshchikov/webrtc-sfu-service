package main

import (
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v4"
)

// WebSocket сообщение для сигналинга
type websocketMessage struct {
	Event string `json:"event"`
	Data  string `json:"data"`
}

// Состояние peer connection с WebSocket
type peerConnectionState struct {
	peerConnection *webrtc.PeerConnection
	websocket      *threadSafeWriter
}

// Потокобезопасная обертка для WebSocket
type threadSafeWriter struct {
	*websocket.Conn
	sync.Mutex
}

func (t *threadSafeWriter) WriteJSON(v interface{}) error {
	t.Lock()
	defer t.Unlock()
	return t.Conn.WriteJSON(v)
}

// Глобальные переменные (для простоты, как в оригинале)
var (
	peerConnections []peerConnectionState
	trackLocals     map[string]*webrtc.TrackLocalStaticRTP
	listLock        sync.RWMutex
)

// Настройка WebSocket upgrader
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}
