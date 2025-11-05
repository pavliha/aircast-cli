package cli

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
)

// Config holds the bridge configuration
type Config struct {
	WebSocketURL string
	AuthToken    string
	TCPAddress   string
	UDPAddress   string
	Logger       *log.Entry
}

// Bridge represents a MAVLink WebSocket-to-TCP/UDP bridge
type Bridge struct {
	config *Config
	logger *log.Entry

	// WebSocket connection
	wsConn   *websocket.Conn
	wsMutex  sync.Mutex
	wsCtx    context.Context
	wsCancel context.CancelFunc

	// TCP listener
	tcpListener net.Listener
	tcpClients  map[string]net.Conn
	tcpMutex    sync.RWMutex

	// UDP listener
	udpConn    *net.UDPConn
	udpClients map[string]*net.UDPAddr
	udpMutex   sync.RWMutex

	// Control
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Circuit breaker for reconnection
	circuitState      string // "closed", "open", "half-open"
	failureCount      int
	lastFailureTime   time.Time
	circuitOpenUntil  time.Time
	failureThreshold  int
	circuitOpenPeriod time.Duration
}

// New creates a new MAVLink bridge
func New(config *Config) (*Bridge, error) {
	if config.Logger == nil {
		config.Logger = log.WithField("component", "bridge")
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Bridge{
		config:            config,
		logger:            config.Logger,
		tcpClients:        make(map[string]net.Conn),
		udpClients:        make(map[string]*net.UDPAddr),
		ctx:               ctx,
		cancel:            cancel,
		circuitState:      "closed",
		failureThreshold:  3,                // Open circuit after 3 failures
		circuitOpenPeriod: 30 * time.Second, // Keep circuit open for 30 seconds
	}, nil
}

// Start starts the bridge
func (b *Bridge) Start() error {
	// Connect to WebSocket
	if err := b.connectWebSocket(); err != nil {
		return fmt.Errorf("failed to connect to WebSocket: %w", err)
	}

	// Start TCP listener if configured
	if b.config.TCPAddress != "" {
		if err := b.startTCPListener(); err != nil {
			return fmt.Errorf("failed to start TCP listener: %w", err)
		}
	}

	// Start UDP listener if configured
	if b.config.UDPAddress != "" {
		if err := b.startUDPListener(); err != nil {
			return fmt.Errorf("failed to start UDP listener: %w", err)
		}
	}

	// Start WebSocket reader
	b.wg.Add(1)
	go b.readWebSocket()

	return nil
}

// Stop stops the bridge
func (b *Bridge) Stop() error {
	b.cancel()

	// Close WebSocket
	if b.wsConn != nil {
		b.wsCancel()
		_ = b.wsConn.Close()
	}

	// Close TCP listener and clients
	if b.tcpListener != nil {
		_ = b.tcpListener.Close()
	}
	b.tcpMutex.Lock()
	for _, conn := range b.tcpClients {
		_ = conn.Close()
	}
	b.tcpMutex.Unlock()

	// Close UDP listener
	if b.udpConn != nil {
		_ = b.udpConn.Close()
	}

	// Wait for goroutines
	b.wg.Wait()

	return nil
}

// connectWebSocket connects to the WebSocket endpoint
func (b *Bridge) connectWebSocket() error {
	b.logger.WithField("url", b.config.WebSocketURL).Info("Connecting to WebSocket")

	// Create WebSocket dialer with auth header
	header := http.Header{}
	if b.config.AuthToken != "" {
		header.Add("Authorization", "Bearer "+b.config.AuthToken)
	}

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, _, err := dialer.Dial(b.config.WebSocketURL, header)
	if err != nil {
		return fmt.Errorf("WebSocket dial failed: %w", err)
	}

	b.wsConn = conn
	b.wsCtx, b.wsCancel = context.WithCancel(b.ctx)

	b.logger.Info("WebSocket connected")
	return nil
}

// startTCPListener starts the TCP listener
func (b *Bridge) startTCPListener() error {
	listener, err := net.Listen("tcp", b.config.TCPAddress)
	if err != nil {
		return fmt.Errorf("failed to listen on TCP %s: %w", b.config.TCPAddress, err)
	}

	b.tcpListener = listener
	b.logger.WithField("address", b.config.TCPAddress).Info("TCP listener started")

	b.wg.Add(1)
	go b.acceptTCPConnections()

	return nil
}

// acceptTCPConnections accepts incoming TCP connections
func (b *Bridge) acceptTCPConnections() {
	defer b.wg.Done()

	for {
		conn, err := b.tcpListener.Accept()
		if err != nil {
			select {
			case <-b.ctx.Done():
				return
			default:
				b.logger.WithError(err).Error("TCP accept error")
				continue
			}
		}

		clientAddr := conn.RemoteAddr().String()
		b.logger.WithField("client", clientAddr).Info("TCP client connected")

		b.tcpMutex.Lock()
		b.tcpClients[clientAddr] = conn
		b.tcpMutex.Unlock()

		b.wg.Add(1)
		go b.handleTCPClient(conn)
	}
}

// handleTCPClient handles a TCP client connection
func (b *Bridge) handleTCPClient(conn net.Conn) {
	defer b.wg.Done()
	clientAddr := conn.RemoteAddr().String()
	logger := b.logger.WithField("tcp_client", clientAddr)

	defer func() {
		_ = conn.Close()
		b.tcpMutex.Lock()
		delete(b.tcpClients, clientAddr)
		b.tcpMutex.Unlock()
		logger.Info("TCP client disconnected")
	}()

	// Read from TCP client and forward to WebSocket
	buf := make([]byte, 4096)
	for {
		select {
		case <-b.ctx.Done():
			return
		default:
		}

		n, err := conn.Read(buf)
		if err != nil {
			if err != io.EOF {
				logger.WithError(err).Debug("TCP read error")
			}
			return
		}

		// Forward to WebSocket
		if err := b.writeToWebSocket(buf[:n]); err != nil {
			logger.WithError(err).Error("Failed to forward TCP data to WebSocket")
			return
		}
	}
}

// startUDPListener starts the UDP listener
func (b *Bridge) startUDPListener() error {
	addr, err := net.ResolveUDPAddr("udp", b.config.UDPAddress)
	if err != nil {
		return fmt.Errorf("failed to resolve UDP address %s: %w", b.config.UDPAddress, err)
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on UDP %s: %w", b.config.UDPAddress, err)
	}

	b.udpConn = conn
	b.logger.WithField("address", b.config.UDPAddress).Info("UDP listener started")

	b.wg.Add(1)
	go b.readUDP()

	return nil
}

// readUDP reads from UDP and forwards to WebSocket
func (b *Bridge) readUDP() {
	defer b.wg.Done()

	buf := make([]byte, 4096)
	for {
		select {
		case <-b.ctx.Done():
			return
		default:
		}

		n, addr, err := b.udpConn.ReadFromUDP(buf)
		if err != nil {
			select {
			case <-b.ctx.Done():
				return
			default:
				b.logger.WithError(err).Error("UDP read error")
				continue
			}
		}

		// Track UDP client
		clientAddr := addr.String()
		b.udpMutex.Lock()
		if _, exists := b.udpClients[clientAddr]; !exists {
			b.udpClients[clientAddr] = addr
			b.logger.WithField("client", clientAddr).Info("UDP client detected")
		}
		b.udpMutex.Unlock()

		// Forward to WebSocket
		if err := b.writeToWebSocket(buf[:n]); err != nil {
			b.logger.WithError(err).Error("Failed to forward UDP data to WebSocket")
		}
	}
}

// readWebSocket reads from WebSocket and forwards to TCP/UDP clients
func (b *Bridge) readWebSocket() {
	defer b.wg.Done()

	for {
		select {
		case <-b.ctx.Done():
			return
		default:
		}

		msgType, data, err := b.wsConn.ReadMessage()
		if err != nil {
			select {
			case <-b.ctx.Done():
				return
			default:
				b.logger.WithError(err).Error("WebSocket read error")
				b.recordFailure()

				// Check circuit breaker state
				if b.circuitState == "open" {
					waitTime := time.Until(b.circuitOpenUntil)
					if waitTime > 0 {
						fmt.Printf("\n⏸️  Device not ready. Waiting %v before retry...\n\n", waitTime.Round(time.Second))

						// Sleep with context cancellation support
						select {
						case <-b.ctx.Done():
							return
						case <-time.After(waitTime):
							b.circuitState = "half-open"
							fmt.Println("🔄 Retrying connection...")
						}
					}
				}

				// Try to reconnect
				if err := b.reconnectWebSocket(); err != nil {
					b.logger.WithError(err).Error("Failed to reconnect WebSocket")
					time.Sleep(2 * time.Second)
				}
				// Don't reset circuit breaker on successful reconnection
				// It will reset only after receiving actual data
				continue
			}
		}

		// Successful data received - reset circuit breaker
		b.resetCircuit()

		// Only process binary messages
		if msgType != websocket.BinaryMessage {
			b.logger.Debug("Ignoring non-binary WebSocket message")
			continue
		}

		// Forward to all TCP clients
		b.tcpMutex.RLock()
		for clientAddr, conn := range b.tcpClients {
			if _, err := conn.Write(data); err != nil {
				b.logger.WithError(err).WithField("client", clientAddr).Error("Failed to write to TCP client")
			}
		}
		b.tcpMutex.RUnlock()

		// Forward to all UDP clients
		if b.udpConn != nil {
			b.udpMutex.RLock()
			for clientAddr, addr := range b.udpClients {
				if _, err := b.udpConn.WriteToUDP(data, addr); err != nil {
					b.logger.WithError(err).WithField("client", clientAddr).Error("Failed to write to UDP client")
				}
			}
			b.udpMutex.RUnlock()
		}
	}
}

// writeToWebSocket writes data to the WebSocket
func (b *Bridge) writeToWebSocket(data []byte) error {
	b.wsMutex.Lock()
	defer b.wsMutex.Unlock()

	if b.wsConn == nil {
		return fmt.Errorf("WebSocket not connected")
	}

	return b.wsConn.WriteMessage(websocket.BinaryMessage, data)
}

// reconnectWebSocket attempts to reconnect to the WebSocket
func (b *Bridge) reconnectWebSocket() error {
	b.wsMutex.Lock()
	defer b.wsMutex.Unlock()

	b.logger.Info("Attempting to reconnect WebSocket")

	// Close old connection
	if b.wsConn != nil {
		_ = b.wsConn.Close()
		b.wsConn = nil
	}

	// Create new connection
	header := http.Header{}
	if b.config.AuthToken != "" {
		header.Add("Authorization", "Bearer "+b.config.AuthToken)
	}

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, _, err := dialer.Dial(b.config.WebSocketURL, header)
	if err != nil {
		return fmt.Errorf("WebSocket reconnect failed: %w", err)
	}

	b.wsConn = conn
	b.logger.Info("WebSocket reconnected")

	return nil
}

// recordFailure records a connection failure and opens circuit if threshold is reached
func (b *Bridge) recordFailure() {
	b.wsMutex.Lock()
	defer b.wsMutex.Unlock()

	b.failureCount++
	b.lastFailureTime = time.Now()

	if b.failureCount >= b.failureThreshold && b.circuitState == "closed" {
		b.circuitState = "open"
		b.circuitOpenUntil = time.Now().Add(b.circuitOpenPeriod)
		fmt.Printf("\n⚠️  Device MAVLink proxy is not running.\n")
		fmt.Printf("   Please start the aircast-agent on your device.\n")
		fmt.Printf("   Retrying in %v...\n\n", b.circuitOpenPeriod)
	}
}

// resetCircuit resets the circuit breaker after successful connection
func (b *Bridge) resetCircuit() {
	b.wsMutex.Lock()
	defer b.wsMutex.Unlock()

	if b.failureCount > 0 {
		fmt.Println("\n✅ Connected! MAVLink data is flowing.\n")
	}
	b.failureCount = 0
	b.circuitState = "closed"
}
