package main

import (
	"encoding/json"
	"log"
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v4"
)

// Инициализация SFU компонентов
func initSFU() {
	trackLocals = map[string]*webrtc.TrackLocalStaticRTP{}

	// Автоматический запрос ключевых кадров каждые 3 секунды
	go keyFrameScheduler()
}

// Планировщик запроса ключевых кадров
func keyFrameScheduler() {
	ticker := time.NewTicker(time.Second * 3)
	defer ticker.Stop()

	for range ticker.C {
		dispatchKeyFrame()
	}
}

// Добавление нового трека в систему
func addTrack(t *webrtc.TrackRemote) *webrtc.TrackLocalStaticRTP {
	listLock.Lock()
	defer func() {
		listLock.Unlock()
		signalPeerConnections()
	}()

	// Создание локального трека с параметрами удаленного
	trackLocal, err := webrtc.NewTrackLocalStaticRTP(
		t.Codec().RTPCodecCapability,
		t.ID(),
		t.StreamID(),
	)
	if err != nil {
		log.Printf("Ошибка создания локального трека: %v", err)
		return nil
	}

	trackLocals[t.ID()] = trackLocal
	log.Printf("Добавлен трек: %s (тип: %s)", t.ID(), t.Kind())

	return trackLocal
}

// Удаление трека из системы
func removeTrack(t *webrtc.TrackLocalStaticRTP) {
	if t == nil {
		return
	}

	listLock.Lock()
	defer func() {
		listLock.Unlock()
		signalPeerConnections()
	}()

	delete(trackLocals, t.ID())
	log.Printf("Удален трек: %s", t.ID())
}

// Синхронизация всех peer connections
func signalPeerConnections() {
	listLock.Lock()
	defer func() {
		listLock.Unlock()
		dispatchKeyFrame()
	}()

	// Функция попытки синхронизации
	attemptSync := func() (tryAgain bool) {
		// Очистка закрытых соединений
		peerConnections = cleanupClosedConnections(peerConnections)

		// Синхронизация каждого активного соединения
		for i := range peerConnections {
			if err := syncPeerConnection(&peerConnections[i]); err != nil {
				log.Printf("Ошибка синхронизации peer connection: %v", err)
				return true // Повторить попытку
			}
		}

		return false
	}

	// Повторные попытки синхронизации с ограничением
	const maxAttempts = 25
	for syncAttempt := 0; syncAttempt < maxAttempts; syncAttempt++ {
		if !attemptSync() {
			break
		}

		// Если достигли максимума попыток, отложить следующую попытку
		if syncAttempt == maxAttempts-1 {
			go func() {
				time.Sleep(time.Second * 3)
				signalPeerConnections()
			}()
			return
		}
	}
}

// Очистка закрытых соединений
func cleanupClosedConnections(connections []peerConnectionState) []peerConnectionState {
	var active []peerConnectionState

	for _, conn := range connections {
		if conn.peerConnection.ConnectionState() != webrtc.PeerConnectionStateClosed {
			active = append(active, conn)
		} else {
			log.Printf("Удалено закрытое соединение")
		}
	}

	return active
}

// Синхронизация отдельного peer connection
func syncPeerConnection(connState *peerConnectionState) error {
	pc := connState.peerConnection
	ws := connState.websocket

	// Анализ существующих отправителей
	existingSenders := make(map[string]bool)

	// Обработка текущих отправителей
	for _, sender := range pc.GetSenders() {
		if sender.Track() == nil {
			continue
		}

		existingSenders[sender.Track().ID()] = true

		// Удаление треков, которых больше нет в реестре
		if _, exists := trackLocals[sender.Track().ID()]; !exists {
			if err := pc.RemoveTrack(sender); err != nil {
				return err
			}
		}
	}

	// Предотвращение петли обратной связи
	for _, receiver := range pc.GetReceivers() {
		if receiver.Track() == nil {
			continue
		}
		existingSenders[receiver.Track().ID()] = true
	}

	// Добавление новых треков
	for trackID := range trackLocals {
		if !existingSenders[trackID] {
			if _, err := pc.AddTrack(trackLocals[trackID]); err != nil {
				return err
			}
		}
	}

	// Создание и отправка offer
	offer, err := pc.CreateOffer(nil)
	if err != nil {
		return err
	}

	if err = pc.SetLocalDescription(offer); err != nil {
		return err
	}

	offerString, err := json.Marshal(offer)
	if err != nil {
		return err
	}

	return ws.WriteJSON(&websocketMessage{
		Event: "offer",
		Data:  string(offerString),
	})
}

// Запрос ключевых кадров от всех участников
func dispatchKeyFrame() {
	listLock.Lock()
	defer listLock.Unlock()

	for i := range peerConnections {
		for _, receiver := range peerConnections[i].peerConnection.GetReceivers() {
			if receiver.Track() == nil {
				continue
			}

			// Отправка Picture Loss Indication для запроса I-frame
			_ = peerConnections[i].peerConnection.WriteRTCP([]rtcp.Packet{
				&rtcp.PictureLossIndication{
					MediaSSRC: uint32(receiver.Track().SSRC()),
				},
			})
		}
	}
}

// Обработчик входящих треков
func handleTrack(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
	log.Printf("Получен удаленный трек: тип=%s, ID=%s, PayloadType=%d",
		track.Kind(), track.ID(), track.PayloadType())

	// Создание локального трека для пересылки
	trackLocal := addTrack(track)
	if trackLocal == nil {
		return
	}

	// Очистка при завершении
	defer removeTrack(trackLocal)

	// Буфер для RTP пакетов
	buf := make([]byte, 1500)
	rtpPkt := &rtp.Packet{}

	// Чтение и пересылка RTP пакетов
	for {
		i, _, err := track.Read(buf)
		if err != nil {
			log.Printf("Ошибка чтения трека %s: %v", track.ID(), err)
			return
		}

		// Демаршалинг RTP пакета
		if err = rtpPkt.Unmarshal(buf[:i]); err != nil {
			log.Printf("Ошибка демаршалинга RTP пакета: %v", err)
			continue
		}

		// Очистка расширений для совместимости
		rtpPkt.Extension = false
		rtpPkt.Extensions = nil

		// Запись в локальный трек для пересылки
		if err = trackLocal.WriteRTP(rtpPkt); err != nil {
			log.Printf("Ошибка записи RTP пакета: %v", err)
			return
		}
	}
}
