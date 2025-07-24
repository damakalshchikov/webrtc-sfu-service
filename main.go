package main

import (
	"log"
	"net/http"
)

func main() {
	// Инициализация SFU компонентов
	initSFU()

	// Настройка HTTP обработчиков
	setupHTTPHandlers()

	// Запуск сервера
	startServer()
}

// Настройка HTTP маршрутов
func setupHTTPHandlers() {
	// Статические файлы (HTML, CSS, JS)
	http.Handle("/", http.FileServer(http.Dir("./static/")))

	// WebSocket endpoint для сигналинга
	http.HandleFunc("/websocket", websocketHandler)

	// Дополнительные endpoints при необходимости
	http.HandleFunc("/health", healthCheckHandler)
}

// Простой health check endpoint
func healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status": "ok", "service": "webrtc-sfu"}`))
}

// Запуск HTTP сервера
func startServer() {
	const addr = ":8080"

	log.Printf("WebRTC SFU сервер запущен на http://localhost%s", addr)
	log.Printf("Откройте несколько вкладок для тестирования")
	log.Printf("Health check: http://localhost%s/health", addr)

	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("Ошибка запуска сервера: %v", err)
	}
}
