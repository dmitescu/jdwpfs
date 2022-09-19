// SPDX-License-Identifier: LGPL-3.0
// Copyright (C) 2022 jdwpfs Authors M. G. Dan

package debug

import (
	"context"
	"fmt"
	"log"
	"sync"

	jdwp "github.com/omerye/gojdb/jdwp"
)

//
// Modifier descriptor
//
type ModifierDescriptor struct {
	Name string
	Kind jdwp.TypeTag
	IsField bool
	ClassId uint64
	ObjectId uint64
}

func (d ModifierDescriptor) ToModifier() jdwp.EventModifier {
	return nil
}

//
// Debugging Event
//
type DebuggingEvent struct {
	Name string
	kind jdwp.EventKind
	suspendPolicy jdwp.SuspendPolicy
	modifierDescriptors map[string]ModifierDescriptor
	hookDescriptors map[string]string
	
	mu sync.RWMutex
	registered bool
	ctx context.Context
	conn *jdwp.Connection
	cancel context.CancelFunc
}

func NewStubDebuggingEvent(name string) *DebuggingEvent {
	return &DebuggingEvent {
		Name: name,
		kind: jdwp.VMDeath,
		suspendPolicy: jdwp.SuspendNone,
		modifierDescriptors: map[string]ModifierDescriptor{},
		hookDescriptors: map[string]string{},

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
	
	e.kind = kind
}

func (e *DebuggingEvent) SetSuspendPolicy(policy jdwp.SuspendPolicy) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.suspendPolicy = policy
}

// TODO maybe sanity checks?
func (e *DebuggingEvent) SetHookDescriptor(name string, target string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()

	_, ok := e.hookDescriptors[name]
	if ok {
		return false
	}

	e.hookDescriptors[name] = target

	return true
}

func (e *DebuggingEvent) RemoveHookDescriptor(name string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()

	_, ok := e.hookDescriptors[name]
	if !ok {
		return false
	}

	delete(e.hookDescriptors, name)

	return true
}

func (e *DebuggingEvent) SetModifier(name string, modifierDescriptor ModifierDescriptor) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.modifierDescriptors[name] = modifierDescriptor

	return nil
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

	return e.kind
}

func (e *DebuggingEvent) GetSuspendPolicy() jdwp.SuspendPolicy {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return e.suspendPolicy
}

func (e *DebuggingEvent) GetHookDescriptors() map[string]string {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var descriptors = map[string]string {}
	for key, value := range e.hookDescriptors {
		descriptors[key] = value
	}
	
	return descriptors
}


func (e *DebuggingEvent) GetModifiers() map[string]ModifierDescriptor {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var descriptors = map[string]ModifierDescriptor{}
	for key, value := range e.modifierDescriptors {
		descriptors[key] = value
	}	
	
	return descriptors
}

func (e *DebuggingEvent) GetRegistered() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return e.registered
}

func (e *DebuggingEvent) DeleteModifier(name string) error {
	e.mu.RLock()
	defer e.mu.RUnlock()
	
	_, ok := e.modifierDescriptors[name]
	if !ok {
		log.Printf("modifier %s cannot be found\n", name)
		return JdwpDebuggingEventError{
			message: fmt.Sprintf("modifier %s not found", name),
		}
	}

	delete(e.modifierDescriptors, name)

	return nil
}

func (e *DebuggingEvent) Run() (context.Context, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	
	eventContext, contextCancel := context.WithCancel(context.Background())
	e.ctx = eventContext
	e.cancel = contextCancel

	var modifiers []jdwp.EventModifier
	for _, descriptor := range e.modifierDescriptors {
		var newModifier jdwp.EventModifier
		switch descriptor.IsField {
		case true:
			newModifier = jdwp.FieldOnlyEventModifier {
				Type: jdwp.ReferenceTypeID(descriptor.ClassId),
				Field: jdwp.FieldID(descriptor.ObjectId),
			}
		case false:
			newModifier = jdwp.LocationOnlyEventModifier(jdwp.Location {
				Type: descriptor.Kind,
				Class: jdwp.ClassID(descriptor.ClassId),
				Method: jdwp.MethodID(descriptor.ObjectId),
				Location: 0,
			})
		}

		modifiers = append(modifiers, newModifier)
	}

	var builder = NewPluginRunnerBuilder()
	for hookName, hookPath := range e.hookDescriptors {
		err := builder.AddLocation(hookName, hookPath)
		if err != nil {
			return nil, err
		}
	}

	runner, err := builder.Build()
	if err != nil {
		log.Printf("unable to load plugins: %s", err)
		return nil, err
	}
	
	hook := func(event jdwp.Event) bool {
		err := runner.Entrypoint(event)
		if err != nil {
			log.Printf("running for event %v caused errors: %s\n", event, err)
			return false
		}
		return true
	}

	go func() {
		err := e.conn.WatchEvents(
			eventContext,
			e.kind,
			e.suspendPolicy,
			hook,
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
