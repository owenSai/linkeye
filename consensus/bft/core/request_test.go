package core

import (
	"math/big"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/linkeye/linkeye/common"
	"github.com/linkeye/linkeye/consensus/bft"
	"github.com/linkeye/linkeye/event"
	"github.com/linkeye/linkeye/log"
	"gopkg.in/karalabe/cookiejar.v2/collections/prque"
)

func TestCheckRequestMsg(t *testing.T) {
	c := &core{
		state: StateAcceptRequest,
		current: newRoundState(&bft.View{
			Sequence: big.NewInt(1),
			Round:    big.NewInt(0),
		}, newTestValidatorSet(4), common.Hash{}, nil, nil, nil),
	}

	// invalid request
	err := c.checkRequestMsg(nil)
	if err != errInvalidMessage {
		t.Errorf("error mismatch: have %v, want %v", err, errInvalidMessage)
	}
	r := &bft.Request{
		Proposal: nil,
	}
	err = c.checkRequestMsg(r)
	if err != errInvalidMessage {
		t.Errorf("error mismatch: have %v, want %v", err, errInvalidMessage)
	}

	// old request
	r = &bft.Request{
		Proposal: makeBlock(0),
	}
	err = c.checkRequestMsg(r)
	if err != errOldMessage {
		t.Errorf("error mismatch: have %v, want %v", err, errOldMessage)
	}

	// future request
	r = &bft.Request{
		Proposal: makeBlock(2),
	}
	err = c.checkRequestMsg(r)
	if err != errFutureMessage {
		t.Errorf("error mismatch: have %v, want %v", err, errFutureMessage)
	}

	// current request
	r = &bft.Request{
		Proposal: makeBlock(1),
	}
	err = c.checkRequestMsg(r)
	if err != nil {
		t.Errorf("error mismatch: have %v, want nil", err)
	}
}

func TestStoreRequestMsg(t *testing.T) {
	backend := &testSystemBackend{
		events: new(event.TypeMux),
	}
	c := &core{
		logger:  log.New("backend", "test", "id", 0),
		backend: backend,
		state:   StateAcceptRequest,
		current: newRoundState(&bft.View{
			Sequence: big.NewInt(0),
			Round:    big.NewInt(0),
		}, newTestValidatorSet(4), common.Hash{}, nil, nil, nil),
		pendingRequests:   prque.New(),
		pendingRequestsMu: new(sync.Mutex),
	}
	requests := []bft.Request{
		{
			Proposal: makeBlock(1),
		},
		{
			Proposal: makeBlock(2),
		},
		{
			Proposal: makeBlock(3),
		},
	}

	c.storeRequestMsg(&requests[1])
	c.storeRequestMsg(&requests[0])
	c.storeRequestMsg(&requests[2])
	if c.pendingRequests.Size() != len(requests) {
		t.Errorf("the size of pending requests mismatch: have %v, want %v", c.pendingRequests.Size(), len(requests))
	}

	c.current.sequence = big.NewInt(3)

	c.subscribeEvents()
	defer c.unsubscribeEvents()

	c.processPendingRequests()

	const timeoutDura = 2 * time.Second
	timeout := time.NewTimer(timeoutDura)
	select {
	case ev := <-c.events.Chan():
		e, ok := ev.Data.(bft.RequestEvent)
		if !ok {
			t.Errorf("unexpected event comes: %v", reflect.TypeOf(ev.Data))
		}
		if e.Proposal.Number().Cmp(requests[2].Proposal.Number()) != 0 {
			t.Errorf("the number of proposal mismatch: have %v, want %v", e.Proposal.Number(), requests[2].Proposal.Number())
		}
	case <-timeout.C:
		t.Error("unexpected timeout occurs")
	}
}
