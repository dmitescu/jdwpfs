// SPDX-License-Identifier: LGPL-3.0
// Copyright (C) 2022 jdwpfs Authors M. G. Dan

package debug

import (
	"context"
	"sync"
	"log"
	"fmt"
	
	jdwp "github.com/omerye/gojdb/jdwp"
)

//
// Debugging Event
//
type DebuggingEvent struct {
	Name string
	Kind jdwp.EventKind
	SuspendPolicy jdwp.SuspendPolicy
	Modifiers map[string]jdwp.EventModifier
	
	hook func(jdwp.Event) bool

	mu sync.RWMutex
	registered bool
	ctx context.Context
	conn *jdwp.Connection
	cancel context.CancelFunc
}

func NewStubDebuggingEvent(name string) *DebuggingEvent {
	return &DebuggingEvent {
		Name: name,
		Kind: jdwp.VMDeath,
		SuspendPolicy: jdwp.SuspendNone,
		Modifiers: map[string]jdwp.EventModifier{},
		hook: nil,

		mu: sync.RWMutex{},
		registered: false,
		ctx: nil, // iff it's running
		conn: nil,
		cancel: nil,
	}
}

func (e *DebuggingEvent) SetKind(kind jdwp.EventKind) {
	e.mu.Lock()
	defer e.mu.Unlock()
	
	e.Kind = kind
}

func (e *DebuggingEvent) SetSuspendPolicy(policy jdwp.SuspendPolicy) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.SuspendPolicy = policy
}

func (e *DebuggingEvent) SetModifier(name string, modifier jdwp.EventModifier) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.Modifiers[name] = modifier
}

func (e *DebuggingEvent) SetRegistered(registered bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.registered = registered
}

func (e *DebuggingEvent) SetCtx(ctx context.Context) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.ctx = ctx
}

func (e *DebuggingEvent) SetConn(conn *jdwp.Connection) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.conn = conn
}

func (e *DebuggingEvent) SetCancel(cancel context.CancelFunc) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.cancel = cancel
}

func (e *DebuggingEvent) GetKind() jdwp.EventKind {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return e.Kind
}

func (e *DebuggingEvent) GetSuspendPolicy() jdwp.SuspendPolicy {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return e.SuspendPolicy
}

func (e *DebuggingEvent) GetModifiers() []jdwp.EventModifier {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var modifiers = []jdwp.EventModifier{}
	for _, modifier := range e.Modifiers {
		modifiers = append(modifiers, modifier)
	}
	
	return modifiers
}

func (e *DebuggingEvent) GetRegistered() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return e.registered
}

func (e *DebuggingEvent) DeleteModifier(name string) error {
	e.mu.RLock()
	defer e.mu.RUnlock()
	
	_, ok := e.Modifiers[name]
	if !ok {
		log.Printf("modifier %s cannot be found\n", name)
		return JdwpDebuggingEventError{
			message: fmt.Sprintf("modifier %s not found", name),
		}
	}

	delete(e.Modifiers, name)

	return nil
}

func (e *DebuggingEvent) Run() (context.Context, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	
	eventContext, contextCancel := context.WithCancel(context.Background())
	e.ctx = eventContext
	e.cancel = contextCancel

	var modifiers []jdwp.EventModifier
	for _, modifier := range e.Modifiers {
		modifiers = append(modifiers, modifier)
	}

	go func() {
		err := e.conn.WatchEvents(
			eventContext,
			e.Kind,
			e.SuspendPolicy,
			e.hook,
			modifiers...)
		if err != nil {
			log.Printf("event %s finished with error: %s\n", e.Name, err)
		} else {
			log.Printf("event %s finished successfully\n", e.Name)
		}
	}()

	return eventContext, nil
}

func (e *DebuggingEvent) Cancel() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	
	if e.ctx == nil {
		return JdwpDebuggingEventError{
			message: fmt.Sprintf("e %s not running\n", e.Name),
		}
	}

	log.Printf("cancelling e %s\n", e.Name)
	e.cancel()

	<-e.ctx.Done()
	log.Printf("e %s cancelled successfully\n", e.Name)

	cancelError := e.ctx.Err()

	e.ctx = nil
	e.cancel = nil

	return cancelError
}

func (e *DebuggingEvent) IsRunning() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.ctx != nil
}
