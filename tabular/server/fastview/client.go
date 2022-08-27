package fastview

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	channerics "github.com/niceyeti/channerics/channels"
	"golang.org/x/sync/errgroup"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 1 * time.Second
	// Maximum message size allowed from peer.
	maxMessageSize = 8192

	// The rate at which ele-updates will be sent to the client, so as not to overburden.
	pubResolution  = time.Millisecond * 100
	pingResolution = time.Millisecond * 200
	// Example code sets this to 10*pingResolution. By definition, it encompasses the number of
	// pings to tolerate losing before concluding the peer is gone.
	pongWait = pingResolution * 4
)

var upgrader = websocket.Upgrader{}

// A client encapsulates a mechanism for publishing updates unidirectionally
// to web clients via websocket. As much as possible I'd like this to represent
// a standard websocket client, including the future capability of reading client
// messages, such as posts (i.e., a client page could monitor key strokes for view commands).
// This client could serve as the basis for a full-fledged server-defined game client,
// whereby the server holds game state (possibly among multiple players) and synchronizes
// idempotent web-client's views with it. Likewise shared realtime data displays.
// Though consider WebRTC (udp) and whether TCP (websockets) per use case.
type client[T any] struct {
	updates <-chan T
	ws      *websock
	rootCtx context.Context
}

// NewClient returns a publisher for sending ui or other updates to clients
// via websocket. Items in the updates chan should represent idempotent update
// objects, such that intervening updates can be discarded when they are received
// too quickly (> pub-rate), and only sending the latest update is sufficient to
// specify the new client state (a ui, for example).
func NewClient[T any](
	updates <-chan T,
	w http.ResponseWriter,
	r *http.Request,
) (*client[T], error) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return nil, err
	}

	return &client[T]{
		updates: updates,
		ws:      NewWebSocket(ws),
		rootCtx: r.Context(),
	}, nil
}

// Sync starts routines to publish incoming updates to the passed client request,
// after upgrading it to a websocket from http. Updates are published at a compiled
// rate; updates received faster than that rate are discarded. This makes this publisher
// best-suited to idempotent updates.
// Sync returns nil upon client disconnect or an error if an unexpected error occurred.
// NOTE: the websocket code exemplifies the externalmost layer in Uncle Bobs architecture: net,
// db, drivers, etc. This should be broken out as such, using his "dependency rule".
// NOTE: taking too long here could block senders on the updates chan; this will surely change
// as code develops, just be mindful of upstream effects.
func (cli *client[T]) Sync() error {
	group, groupCtx := errgroup.WithContext(cli.rootCtx)

	group.Go(func() error {
		return cli.readMessages(groupCtx)
	})
	group.Go(func() error {
		return cli.pingPong(groupCtx)
	})
	group.Go(func() error {
		return cli.publish(groupCtx)
	})

	return group.Wait()
}

var ErrPongDeadlineExceeded error = errors.New("client disconnect, pong deadline exceeded")

// Runs the ping-pong for the client liveness check.
// NOTE: This function requires that readPump is running to ensure the pong handler is called.
func (cli *client[T]) pingPong(ctx context.Context) error {
	pong := make(chan struct{})
	defer close(pong)
	cli.ws.Conn().SetPongHandler(func(_ string) error {
		pong <- struct{}{}
		return nil
	})

	pinger := channerics.NewTicker(ctx.Done(), pingResolution)
	lastPong := time.Now()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-pinger:
			if time.Since(lastPong) > pongWait {
				return ErrPongDeadlineExceeded
			}

			if err := cli.ping(ctx); err != nil {
				return err
			}
		case <-pong:
			lastPong = time.Now()
		}
	}
}

func (cli *client[T]) ping(ctx context.Context) error {
	return cli.ws.Write(
		ctx,
		func(ws *websocket.Conn) (err error) {
			if err = ws.WriteControl(websocket.PingMessage, nil, time.Now().Add(writeWait)); err != nil {
				if isError(err) {
					err = fmt.Errorf("ping failed: %T %v", err, err)
				}
			}
			return
		})
}

// readMessages monitors for messages from the client.
// Errors returned by websocket Read methods are permanent, hence any error
// must trigger full teardown.
func (cli *client[T]) readMessages(ctx context.Context) error {
	for {
		// FUTURE: this is where it would be easy to implement a bidirectional @client by merely
		// passing received messages to an output chan of messages from the client.
		err := cli.ws.Read(
			ctx,
			func(ws *websocket.Conn) (readErr error) {
				_, _, readErr = ws.ReadMessage()
				return
			})
		if err != nil {
			return err
		}
	}
}

func (cli *client[T]) publish(ctx context.Context) error {
	lastSync := time.Now()

	for {
		select {
		case <-ctx.Done():
			return nil
		case updates, ok := <-cli.updates:
			// Graceful input channel closure
			if !ok {
				return nil
			}
			// Drop updates when receiving too quickly.
			if time.Since(lastSync) < pubResolution {
				break
			}

			lastSync = time.Now()
			err := cli.ws.Write(
				ctx,
				func(ws *websocket.Conn) (writeErr error) {
					if writeErr = ws.SetWriteDeadline(time.Now().Add(writeWait)); writeErr != nil {
						writeErr = fmt.Errorf("failed to set deadline: %T %w", writeErr, writeErr)
						return
					}

					if writeErr = ws.WriteJSON(updates); writeErr != nil {
						if isError(writeErr) {
							writeErr = fmt.Errorf("publish failed: %T %v", writeErr, writeErr)
						}
					}
					return
				})
			if err != nil {
				return err
			}
		}
	}
}

func isError(err error) bool {
	return err != nil && websocket.IsUnexpectedCloseError(
		err,
		websocket.CloseNormalClosure,
		websocket.CloseGoingAway)
}

func isClosure(err error) bool {
	return err != nil && websocket.IsCloseError(
		err,
		websocket.CloseNormalClosure,
		websocket.CloseGoingAway)
}

// ErrSockCongestion indicates there are too many waiters on the socket for a given op.
var ErrSockCongestion = errors.New("sock op failed due to congestion")

const (
	readDeadline     = time.Second
	writeDeadline    = time.Second
	closeGracePeriod = 10 * time.Second
)

// websock merely serializes reads and writes to the websocket, whose requirements
// are that there may be only one concurrent read and writer at a time.
type websock struct {
	// These are merely mutexes, but channel semantics are cleaner.
	readSem  chan struct{}
	writeSem chan struct{}
	ws       *websocket.Conn
}

func NewWebSocket(ws *websocket.Conn) *websock {
	return &websock{
		readSem:  make(chan struct{}, 1),
		writeSem: make(chan struct{}, 1),
		ws:       ws,
	}
}

// Returns the underlying websocket.
// This should only be used non-concurrently for setup, e.g. adding handlers.
func (sock *websock) Conn() *websocket.Conn {
	return sock.ws
}

// Closes the websocket. This should only be called once no further read/writers exist.
func (sock *websock) Close() {
	sock.readSem <- struct{}{}
	sock.writeSem <- struct{}{}

	_ = sock.ws.SetWriteDeadline(time.Now().Add(writeWait))
	_ = sock.ws.WriteMessage(
		websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	time.Sleep(closeGracePeriod)
	sock.ws.Close()
}

// Read serializes read operations on the internal web socket.
func (sock *websock) Read(
	ctx context.Context,
	readFn func(*websocket.Conn) error,
) error {
	select {
	case <-ctx.Done():
		return nil
	case sock.readSem <- struct{}{}:
		defer func() { <-sock.readSem }()
		return readFn(sock.ws)
	case <-time.After(readDeadline):
		return ErrSockCongestion
	}
}

// Write serializes write operations to the websocket.
func (sock *websock) Write(
	ctx context.Context,
	writeFn func(*websocket.Conn) error,
) error {
	select {
	case <-ctx.Done():
		return nil
	case sock.writeSem <- struct{}{}:
		defer func() { <-sock.writeSem }()
		return writeFn(sock.ws)
	case <-time.After(writeDeadline):
		return ErrSockCongestion
	}
}
