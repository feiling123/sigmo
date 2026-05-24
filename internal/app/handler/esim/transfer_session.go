//go:build esim_transfer

package esim

import (
	"context"
	"sync"

	"github.com/gorilla/websocket"
)

const (
	wsTypeTransferStart          = "start"
	wsTypeTransferUserInput      = "user_input"
	wsTypeTransferSourceDeletion = "source_deletion"
	wsTypeTransferCompleted      = "completed"
	wsTypeTransferError          = "error"
)

type transferSession struct {
	conn           *websocket.Conn
	disconnectCh   chan struct{}
	disconnectOnce sync.Once
	startCh        chan transferClientMessage
	inputCh        chan transferClientMessage
	deleteCh       chan transferClientMessage
}

func newTransferSession(conn *websocket.Conn, cancel context.CancelFunc) *transferSession {
	session := &transferSession{
		conn:         conn,
		disconnectCh: make(chan struct{}),
		startCh:      make(chan transferClientMessage, 1),
		inputCh:      make(chan transferClientMessage, 1),
		deleteCh:     make(chan transferClientMessage, 1),
	}
	go session.readLoop(cancel)
	return session
}

func (s *transferSession) disconnect() {
	s.disconnectOnce.Do(func() {
		close(s.disconnectCh)
	})
}

func (s *transferSession) readLoop(cancel context.CancelFunc) {
	defer s.disconnect()
	for {
		var msg transferClientMessage
		if err := s.conn.ReadJSON(&msg); err != nil {
			return
		}
		switch msg.Type {
		case wsTypeTransferStart:
			sendLatest(s.startCh, msg)
		case wsTypeTransferUserInput:
			sendLatest(s.inputCh, msg)
		case wsTypeTransferSourceDeletion:
			sendLatest(s.deleteCh, msg)
		case wsTypeCancel:
			cancel()
		}
	}
}

func sendLatest(ch chan transferClientMessage, msg transferClientMessage) {
	select {
	case ch <- msg:
	default:
		select {
		case <-ch:
		default:
		}
		ch <- msg
	}
}

func (s *transferSession) send(msg transferServerMessage) error {
	if err := s.conn.WriteJSON(msg); err != nil {
		s.disconnect()
		return err
	}
	return nil
}

func (s *transferSession) sendIfConnected(msg transferServerMessage) {
	select {
	case <-s.disconnectCh:
		return
	default:
	}
	_ = s.send(msg)
}

func (s *transferSession) waitForStart(ctx context.Context) (transferClientMessage, bool) {
	select {
	case msg := <-s.startCh:
		return msg, true
	case <-ctx.Done():
		return transferClientMessage{}, false
	case <-s.disconnectCh:
		return transferClientMessage{}, false
	}
}

func (s *transferSession) waitForUserInput(ctx context.Context) (transferClientMessage, bool) {
	select {
	case msg := <-s.inputCh:
		return msg, true
	case <-ctx.Done():
		return transferClientMessage{}, false
	case <-s.disconnectCh:
		return transferClientMessage{}, false
	}
}

func (s *transferSession) waitForSourceDeletion(ctx context.Context) (transferClientMessage, bool) {
	select {
	case msg := <-s.deleteCh:
		return msg, true
	case <-ctx.Done():
		return transferClientMessage{}, false
	case <-s.disconnectCh:
		return transferClientMessage{}, false
	}
}
