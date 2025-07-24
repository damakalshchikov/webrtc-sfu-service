class WebRTCSFUClient {
    constructor() {
        this.pc = null;
        this.ws = null;
        this.localStream = null;
        this.remoteStreams = new Map();
        this.connectionStartTime = null;
        this.isVideoEnabled = true;
        this.isAudioEnabled = true;

        // DOM элементы
        this.elements = {
            joinBtn: document.getElementById('joinBtn'),
            leaveBtn: document.getElementById('leaveBtn'),
            toggleVideo: document.getElementById('toggleVideo'),
            toggleAudio: document.getElementById('toggleAudio'),
            localVideo: document.getElementById('localVideo'),
            remoteVideos: document.getElementById('remoteVideos'),
            noParticipants: document.getElementById('noParticipants'),
            connectionStatus: document.getElementById('connectionStatus'),
            participantCount: document.getElementById('participantCount'),
            logContainer: document.getElementById('logContainer'),
            clearLogsBtn: document.getElementById('clearLogsBtn'),
            showStatsBtn: document.getElementById('showStatsBtn'),
            statsPanel: document.getElementById('statsPanel'),
            toggleStats: document.getElementById('toggleStats'),
            connectionState: document.getElementById('connectionState'),
            iceState: document.getElementById('iceState'),
            connectionTime: document.getElementById('connectionTime'),
            localAudioStatus: document.getElementById('localAudioStatus'),
            localVideoStatus: document.getElementById('localVideoStatus')
        };

        this.initializeEventListeners();
        this.log('Клиент инициализирован', 'info');
    }

    initializeEventListeners() {
        this.elements.joinBtn.addEventListener('click', () => this.connect());
        this.elements.leaveBtn.addEventListener('click', () => this.disconnect());
        this.elements.toggleVideo.addEventListener('click', () => this.toggleVideo());
        this.elements.toggleAudio.addEventListener('click', () => this.toggleAudio());
        this.elements.clearLogsBtn.addEventListener('click', () => this.clearLogs());
        this.elements.showStatsBtn.addEventListener('click', () => this.toggleStatsPanel());
        this.elements.toggleStats.addEventListener('click', () => this.toggleStatsPanel());

        // Обработка закрытия страницы
        window.addEventListener('beforeunload', () => {
            this.disconnect();
        });
    }

    async connect() {
        try {
            this.log('Начало подключения...', 'info');
            this.updateConnectionStatus('connecting', 'Подключение...');

            await this.initializePeerConnection();
            await this.getUserMedia();
            await this.connectWebSocket();

            this.connectionStartTime = Date.now();
            this.updateUI('connected');

        } catch (error) {
            this.log(`Ошибка подключения: ${error.message}`, 'error');
            this.updateConnectionStatus('error', 'Ошибка подключения');
            this.updateUI('disconnected');
        }
    }

    async initializePeerConnection() {
        this.pc = new RTCPeerConnection({
            iceServers: [
                { urls: 'stun:stun.l.google.com:19302' },
                { urls: 'stun:stun1.l.google.com:19302' }
            ]
        });

        // Обработчики событий
        this.pc.addEventListener('track', (event) => this.handleRemoteTrack(event));
        this.pc.addEventListener('icecandidate', (event) => this.handleIceCandidate(event));
        this.pc.addEventListener('connectionstatechange', () => this.handleConnectionStateChange());
        this.pc.addEventListener('iceconnectionstatechange', () => this.handleIceStateChange());

        this.log('PeerConnection создан', 'success');
    }

    async getUserMedia() {
        try {
            this.localStream = await navigator.mediaDevices.getUserMedia({
                video: { width: 640, height: 480 },
                audio: true
            });

            // Добавление треков в PeerConnection
            this.localStream.getTracks().forEach(track => {
                this.pc.addTrack(track, this.localStream);
            });

            // Отображение локального видео
            this.elements.localVideo.srcObject = this.localStream;

            this.log('Медиапоток получен', 'success');
            this.updateMediaStatus();

        } catch (error) {
            throw new Error(`Не удалось получить доступ к камере/микрофону: ${error.message}`);
        }
    }

    async connectWebSocket() {
        return new Promise((resolve, reject) => {
            const wsUrl = `ws://${window.location.host}/websocket`;
            this.ws = new WebSocket(wsUrl);

            this.ws.onopen = () => {
                this.log('WebSocket подключен', 'success');
                this.updateConnectionStatus('connected', 'Подключен');
                resolve();
            };

            this.ws.onmessage = (event) => this.handleWebSocketMessage(event);

            this.ws.onerror = (error) => {
                this.log('Ошибка WebSocket', 'error');
                reject(new Error('WebSocket connection failed'));
            };

            this.ws.onclose = () => {
                this.log('WebSocket соединение закрыто', 'warning');
                this.updateConnectionStatus('offline', 'Не подключен');
            };

            // Таймаут подключения
            setTimeout(() => {
                if (this.ws.readyState !== WebSocket.OPEN) {
                    reject(new Error('WebSocket connection timeout'));
                }
            }, 5000);
        });
    }

    async handleWebSocketMessage(event) {
        try {
            const message = JSON.parse(event.data);

            switch (message.event) {
                case 'offer':
                    await this.handleOffer(JSON.parse(message.data));
                    break;
                case 'candidate':
                    await this.handleRemoteCandidate(JSON.parse(message.data));
                    break;
                default:
                    this.log(`Неизвестное сообщение: ${message.event}`, 'warning');
            }
        } catch (error) {
            this.log(`Ошибка обработки сообщения: ${error.message}`, 'error');
        }
    }

    async handleOffer(offer) {
        try {
            await this.pc.setRemoteDescription(offer);
            const answer = await this.pc.createAnswer();
            await this.pc.setLocalDescription(answer);

            this.sendWebSocketMessage('answer', JSON.stringify(answer));
            this.log('Отправлен answer', 'info');

        } catch (error) {
            this.log(`Ошибка обработки offer: ${error.message}`, 'error');
        }
    }

    async handleRemoteCandidate(candidate) {
        try {
            await this.pc.addIceCandidate(candidate);
        } catch (error) {
            this.log(`Ошибка добавления ICE кандидата: ${error.message}`, 'error');
        }
    }

    handleIceCandidate(event) {
        if (event.candidate && this.ws && this.ws.readyState === WebSocket.OPEN) {
            this.sendWebSocketMessage('candidate', JSON.stringify(event.candidate));
        }
    }

    handleRemoteTrack(event) {
        const [stream] = event.streams;
        const track = event.track;

        if (track.kind === 'video') {
            this.addRemoteVideo(stream, track.id);
            this.log(`Получен удаленный видеопоток: ${track.id}`, 'success');
        }

        this.updateParticipantCount();
    }

    addRemoteVideo(stream, trackId) {
        // Удаление существующего видео с таким ID
        this.removeRemoteVideo(trackId);

        const videoWrapper = document.createElement('div');
        videoWrapper.className = 'video-wrapper remote';
        videoWrapper.id = `remote-${trackId}`;

        const video = document.createElement('video');
        video.srcObject = stream;
        video.autoplay = true;
        video.playsinline = true;

        const overlay = document.createElement('div');
        overlay.className = 'video-overlay';
        overlay.innerHTML = `
            <span class="video-label">Участник</span>
            <div class="video-controls">
                <span class="connection-quality">📶</span>
            </div>
        `;

        videoWrapper.appendChild(video);
        videoWrapper.appendChild(overlay);
        this.elements.remoteVideos.appendChild(videoWrapper);

        this.remoteStreams.set(trackId, { stream, video, wrapper: videoWrapper });
        this.updateRemoteVideosVisibility();
    }

    removeRemoteVideo(trackId) {
        const existing = document.getElementById(`remote-${trackId}`);
        if (existing) {
            existing.remove();
        }
        this.remoteStreams.delete(trackId);
        this.updateRemoteVideosVisibility();
        this.updateParticipantCount();
    }

    updateRemoteVideosVisibility() {
        const hasRemoteVideos = this.remoteStreams.size > 0;
        this.elements.noParticipants.style.display = hasRemoteVideos ? 'none' : 'flex';
    }

    updateParticipantCount() {
        const count = this.remoteStreams.size;
        this.elements.participantCount.textContent = count;
    }

    handleConnectionStateChange() {
        const state = this.pc.connectionState;
        this.elements.connectionState.textContent = state;
        this.log(`Состояние соединения: ${state}`, 'info');

        if (state === 'failed' || state === 'closed') {
            this.updateConnectionStatus('error', 'Соединение потеряно');
        }
    }

    handleIceStateChange() {
        const state = this.pc.iceConnectionState;
        this.elements.iceState.textContent = state;
        this.log(`ICE состояние: ${state}`, 'info');
    }

    async toggleVideo() {
        if (!this.localStream) return;

        const videoTrack = this.localStream.getVideoTracks()[0];
        if (videoTrack) {
            this.isVideoEnabled = !this.isVideoEnabled;
            videoTrack.enabled = this.isVideoEnabled;
            this.updateMediaControls();
            this.updateMediaStatus();

            this.log(`Видео ${this.isVideoEnabled ? 'включено' : 'выключено'}`, 'info');
        }
    }

    async toggleAudio() {
        if (!this.localStream) return;

        const audioTrack = this.localStream.getAudioTracks()[0];
        if (audioTrack) {
            this.isAudioEnabled = !this.isAudioEnabled;
            audioTrack.enabled = this.isAudioEnabled;
            this.updateMediaControls();
            this.updateMediaStatus();

            this.log(`Аудио ${this.isAudioEnabled ? 'включено' : 'выключено'}`, 'info');
        }
    }

    updateMediaControls() {
        this.elements.toggleVideo.classList.toggle('disabled', !this.isVideoEnabled);
        this.elements.toggleAudio.classList.toggle('disabled', !this.isAudioEnabled);

        const videoIcon = this.elements.toggleVideo.querySelector('.btn-icon');
        const audioIcon = this.elements.toggleAudio.querySelector('.btn-icon');

        videoIcon.textContent = this.isVideoEnabled ? '📹' : '📹❌';
        audioIcon.textContent = this.isAudioEnabled ? '🎤' : '🎤❌';
    }

    updateMediaStatus() {
        this.elements.localVideoStatus.textContent = this.isVideoEnabled ? '📹' : '📹❌';
        this.elements.localAudioStatus.textContent = this.isAudioEnabled ? '🎤' : '🎤❌';
    }

    disconnect() {
        this.log('Отключение...', 'info');

        // Остановка локального медиапотока
        if (this.localStream) {
            this.localStream.getTracks().forEach(track => track.stop());
            this.localStream = null;
        }

        // Закрытие PeerConnection
        if (this.pc) {
            this.pc.close();
            this.pc = null;
        }

        // Закрытие WebSocket
        if (this.ws) {
            this.ws.close();
            this.ws = null;
        }

        // Очистка UI
        this.elements.localVideo.srcObject = null;
        this.elements.remoteVideos.innerHTML = '';
        this.remoteStreams.clear();

        this.updateUI('disconnected');
        this.updateConnectionStatus('offline', 'Не подключен');
        this.updateParticipantCount();
        this.updateRemoteVideosVisibility();

        this.log('Отключен', 'info');
    }

    updateUI(state) {
        const isConnected = state === 'connected';
        const isConnecting = state === 'connecting';

        this.elements.joinBtn.disabled = isConnected || isConnecting;
        this.elements.leaveBtn.disabled = !isConnected;
        this.elements.toggleVideo.disabled = !isConnected;
        this.elements.toggleAudio.disabled = !isConnected;

        if (isConnecting) {
            this.elements.joinBtn.innerHTML = '<span class="btn-icon">⏳</span>Подключение...';
        } else {
            this.elements.joinBtn.innerHTML = '<span class="btn-icon">🔗</span>Подключиться';
        }
    }

    updateConnectionStatus(status, text) {
        this.elements.connectionStatus.className = `status-indicator ${status}`;
        this.elements.connectionStatus.textContent = text;

        // Обновление времени подключения
        if (status === 'connected' && this.connectionStartTime) {
            this.updateConnectionTime();
        }
    }

    updateConnectionTime() {
        if (!this.connectionStartTime) return;

        const elapsed = Math.floor((Date.now() - this.connectionStartTime) / 1000);
        const minutes = Math.floor(elapsed / 60);
        const seconds = elapsed % 60;

        this.elements.connectionTime.textContent =
            `${minutes}:${seconds.toString().padStart(2, '0')}`;
    }

    sendWebSocketMessage(event, data) {
        if (this.ws && this.ws.readyState === WebSocket.OPEN) {
            this.ws.send(JSON.stringify({ event, data }));
        }
    }

    log(message, level = 'info') {
        const timestamp = new Date().toLocaleTimeString();
        const logEntry = document.createElement('div');
        logEntry.className = `log-entry ${level}`;
        logEntry.innerHTML = `
            <span class="log-time">${timestamp}</span>
            <span class="log-message">${message}</span>
        `;

        this.elements.logContainer.appendChild(logEntry);
        this.elements.logContainer.scrollTop = this.elements.logContainer.scrollHeight;

        // Ограничение количества логов
        const logs = this.elements.logContainer.children;
        if (logs.length > 100) {
            logs[0].remove();
        }

        console.log(`[${level.toUpperCase()}] ${message}`);
    }

    clearLogs() {
        this.elements.logContainer.innerHTML = '';
        this.log('Логи очищены', 'info');
    }

    toggleStatsPanel() {
        this.elements.statsPanel.classList.toggle('hidden');

        if (!this.elements.statsPanel.classList.contains('hidden')) {
            this.updateStats();
            // Обновление статистики каждые 2 секунды
            this.statsInterval = setInterval(() => this.updateStats(), 2000);
        } else if (this.statsInterval) {
            clearInterval(this.statsInterval);
        }
    }

    updateStats() {
        if (this.pc) {
            this.elements.connectionState.textContent = this.pc.connectionState;
            this.elements.iceState.textContent = this.pc.iceConnectionState;

            if (this.connectionStartTime) {
                this.updateConnectionTime();
            }
        }
    }
}

// Инициализация клиента при загрузке страницы
document.addEventListener('DOMContentLoaded', () => {
    window.sfuClient = new WebRTCSFUClient();

    // Обновление времени соединения каждую секунду
    setInterval(() => {
        if (window.sfuClient.connectionStartTime) {
            window.sfuClient.updateConnectionTime();
        }
    }, 1000);
});
