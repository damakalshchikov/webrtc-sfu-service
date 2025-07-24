# WebRTC SFU Service

A scalable Selective Forwarding Unit (SFU) service for WebRTC applications.

## Overview

This service provides a WebRTC SFU implementation that allows for efficient media routing in real-time communication applications. The SFU acts as an intermediary that receives media streams from multiple participants and selectively forwards them to other participants, reducing the bandwidth requirements compared to a full mesh topology.

## Features

- Scalable architecture for handling multiple participants
- Selective forwarding of audio/video streams
- Support for multiple codecs
- RESTful API for session management
- WebSocket signaling for real-time communication

## Getting Started

### Prerequisites

- Go 1.19 or higher
- Docker (optional, for containerized deployment)

### Installation

```bash
# Clone the repository
git clone https://github.com/damakalshchikov/webrtc-sfu-service.git
cd webrtc-sfu-service

# Install dependencies
go mod tidy

# Build the service
go build -o sfu-service

# Run the service
./sfu-service
```

### Configuration

The service can be configured using environment variables:

- `LISTEN_ADDR`: Address to listen on (default: ":8080")
- `TURN_SERVER`: TURN server URL (if required)
- `STUN_SERVER`: STUN server URL (default: "stun:stun.l.google.com:19302")

## API Documentation

Detailed API documentation can be found in the [API Docs](docs/api.md).

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/AmazingFeature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/AmazingFeature`)
5. Open a pull request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Contact

Project maintainer - [@damakalshchikov](https://github.com/damakalshchikov)
