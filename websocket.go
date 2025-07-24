package main

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/pion/webrtc/v4"
)

// Обработчик WebSocket соединений
func websocketHandler(w http.ResponseWriter, r *http.Request) {
	// Upgrade HTTP соединения до WebSocket
	unsafeConn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Ошибка upgrade соединения: %v", err)
		return
	}

	// Создание потокобезопасной обертки
	c := &threadSafeWriter{unsafeConn, sync.Mutex{}}
	defer c.Close()

	log.Printf("Новое WebSocket соединение от %s", r.RemoteAddr)

	// Создание WebRTC peer connection
	peerConnection, err := createPeerConnection()
	if err != nil {
		log.Printf("Ошибка создания PeerConnection: %v", err)
		return
	}
	defer peerConnection.Close()

	// Настройка обработчиков peer connection
	setupPeerConnectionHandlers(peerConnection, c)

	// Добавление соединения в глобальный список
	addPeerConnection(peerConnection, c)

	// Запуск начальной синхронизации
	signalPeerConnections()

	// Основной цикл обработки WebSocket сообщений
	handleWebSocketMessages(c, peerConnection)
}

// Создание и настройка WebRTC peer connection
func createPeerConnection() (*webrtc.PeerConnection, error) {
	// Базовая конфигурация с STUN сервером
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	}

	peerConnection, err := webrtc.NewPeerConnection(config)
	if err != nil {
		return nil, err
	}

	// Настройка трансиверов для приема видео и аудио
	for _, codecType := range []webrtc.RTPCodecType{
		webrtc.RTPCodecTypeVideo,
		webrtc.RTPCodecTypeAudio,
	} {
		_, err := peerConnection.AddTransceiverFromKind(codecType, webrtc.RTPTransceiverInit{
			Direction: webrtc.RTPTransceiverDirectionRecvonly,
		})
		if err != nil {
			peerConnection.Close()
			return nil, err
		}
	}

	return peerConnection, nil
}

// Настройка обработчиков событий peer connection
func setupPeerConnectionHandlers(pc *webrtc.PeerConnection, ws *threadSafeWriter) {
	// Обработка ICE кандидатов
	pc.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if candidate == nil {
			return
		}

		candidateString, err := json.Marshal(candidate.ToJSON())
		if err != nil {
			log.Printf("Ошибка маршалинга ICE кандидата: %v", err)
			return
		}

		if err := ws.WriteJSON(&websocketMessage{
			Event: "candidate",
			Data:  string(candidateString),
		}); err != nil {
			log.Printf("Ошибка отправки ICE кандидата: %v", err)
		}
	})

	// Обработка изменения состояния соединения
	pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		log.Printf("Состояние соединения изменилось: %s", state.String())

		switch state {
		case webrtc.PeerConnectionStateFailed:
			if err := pc.Close(); err != nil {
				log.Printf("Ошибка закрытия failed соединения: %v", err)
			}
		case webrtc.PeerConnectionStateClosed:
			// Запуск пересинхронизации при закрытии соединения
			go signalPeerConnections()
		}
	})

	// Обработка входящих треков
	pc.OnTrack(handleTrack)
}

// Добавление peer connection в глобальный список
func addPeerConnection(pc *webrtc.PeerConnection, ws *threadSafeWriter) {
	listLock.Lock()
	defer listLock.Unlock()

	peerConnections = append(peerConnections, peerConnectionState{
		peerConnection: pc,
		websocket:      ws,
	})

	log.Printf("Добавлено соединение, всего активных: %d", len(peerConnections))
}

// Обработка WebSocket сообщений
func handleWebSocketMessages(ws *threadSafeWriter, pc *webrtc.PeerConnection) {
	message := &websocketMessage{}

	for {
		// Чтение сообщения
		_, raw, err := ws.ReadMessage()
		if err != nil {
			log.Printf("Ошибка чтения WebSocket сообщения: %v", err)
			return
		}

		// Демаршалинг сообщения
		if err := json.Unmarshal(raw, message); err != nil {
			log.Printf("Ошибка демаршалинга сообщения: %v", err)
			continue
		}

		// Обработка по типу события
		if err := processWebSocketMessage(message, pc); err != nil {
			log.Printf("Ошибка обработки сообщения %s: %v", message.Event, err)
			return
		}
	}
}

// Обработка конкретного WebSocket сообщения
func processWebSocketMessage(msg *websocketMessage, pc *webrtc.PeerConnection) error {
	switch msg.Event {
	case "candidate":
		return handleCandidateMessage(msg.Data, pc)
	case "answer":
		return handleAnswerMessage(msg.Data, pc)
	default:
		log.Printf("Неизвестный тип сообщения: %s", msg.Event)
		return nil
	}
}

// Обработка ICE кандидата от клиента
func handleCandidateMessage(data string, pc *webrtc.PeerConnection) error {
	candidate := webrtc.ICECandidateInit{}
	if err := json.Unmarshal([]byte(data), &candidate); err != nil {
		return err
	}

	return pc.AddICECandidate(candidate)
}

// Обработка answer от клиента
func handleAnswerMessage(data string, pc *webrtc.PeerConnection) error {
	answer := webrtc.SessionDescription{}
	if err := json.Unmarshal([]byte(data), &answer); err != nil {
		return err
	}

	return pc.SetRemoteDescription(answer)
}
