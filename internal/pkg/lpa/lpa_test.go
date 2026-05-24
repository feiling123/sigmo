package lpa

import (
	"errors"
	"testing"
	"time"
)

func TestLockedChannelDisconnectOnce(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		channel *fakeSmartCardChannel
		wantErr error
	}{
		{
			name:    "disconnect succeeds once",
			channel: &fakeSmartCardChannel{},
		},
		{
			name:    "disconnect error is returned once",
			channel: &fakeSmartCardChannel{disconnectErr: errFakeDisconnect},
			wantErr: errFakeDisconnect,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := "test:" + tt.name
			gmu.Lock(key)
			channel := &lockedChannel{SmartCardChannel: tt.channel, key: key}

			err := channel.Disconnect()
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Disconnect() error = %v, want %v", err, tt.wantErr)
			}
			if err := channel.Disconnect(); err != nil {
				t.Fatalf("second Disconnect() error = %v", err)
			}
			if tt.channel.disconnects != 1 {
				t.Fatalf("disconnects = %d, want 1", tt.channel.disconnects)
			}
			assertLockReleased(t, key)
		})
	}
}

func TestLockedChannelCloseLogicalChannelReleasesOnError(t *testing.T) {
	t.Parallel()

	key := "test:close-logical-channel-error"
	gmu.Lock(key)
	channel := &fakeSmartCardChannel{closeLogicalChannelErr: errFakeCloseLogicalChannel}
	locked := &lockedChannel{SmartCardChannel: channel, key: key}

	err := locked.CloseLogicalChannel(1)
	if !errors.Is(err, errFakeCloseLogicalChannel) {
		t.Fatalf("CloseLogicalChannel() error = %v, want %v", err, errFakeCloseLogicalChannel)
	}
	if channel.disconnects != 1 {
		t.Fatalf("disconnects = %d, want 1", channel.disconnects)
	}
	if err := locked.Disconnect(); err != nil {
		t.Fatalf("Disconnect() error = %v", err)
	}
	if channel.disconnects != 1 {
		t.Fatalf("disconnects after second release = %d, want 1", channel.disconnects)
	}
	assertLockReleased(t, key)
}

func assertLockReleased(t *testing.T, key string) {
	t.Helper()

	acquired := make(chan struct{})
	go func() {
		gmu.Lock(key)
		defer gmu.Unlock(key)
		close(acquired)
	}()

	select {
	case <-acquired:
	case <-time.After(time.Second):
		t.Fatal("lock was not released")
	}
}

var errFakeDisconnect = errors.New("disconnect")
var errFakeCloseLogicalChannel = errors.New("close logical channel")

type fakeSmartCardChannel struct {
	disconnectErr          error
	closeLogicalChannelErr error
	disconnects            int
}

func (f *fakeSmartCardChannel) Connect() error {
	return nil
}

func (f *fakeSmartCardChannel) Disconnect() error {
	f.disconnects++
	return f.disconnectErr
}

func (f *fakeSmartCardChannel) OpenLogicalChannel([]byte) (byte, error) {
	return 0, nil
}

func (f *fakeSmartCardChannel) Transmit([]byte) ([]byte, error) {
	return nil, nil
}

func (f *fakeSmartCardChannel) CloseLogicalChannel(byte) error {
	return f.closeLogicalChannelErr
}
