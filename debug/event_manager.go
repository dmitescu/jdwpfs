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
// Debugging event errors
//
type JdwpDebuggingEventError struct {
	err error
	message string
}

func (e JdwpDebuggingEventError) Error() string {
	if e.err != nil {
		return fmt.Sprintf("jdwp event dir error: %s", e.err)
	}

	return fmt.Sprintf("jdwp event dir error: %s", e.message)
}

//
// Debugging Event Manager
//
type EventManager struct {		
	JdwpContext context.Context
	JdwpConnection *jdwp.Connection

	mu sync.RWMutex
	registeredEvents []*DebuggingEvent
}

func NewEventManager(ctx context.Context, conn *jdwp.Connection) (*EventManager, error) {
	manager := &EventManager {
		JdwpContext: ctx,
		JdwpConnection: conn,
		mu: sync.RWMutex{},
	}

	return manager, nil
}

func (m *EventManager) CreateEvent(name string) (*DebuggingEvent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var eventFound bool = false
	for _, foundEvent := range m.registeredEvents {
		if foundEvent.Name == name {
			eventFound = true
		}
	}

	if eventFound {
		log.Printf("event with name %s already exists\n", name)
		return nil, JdwpDebuggingEventError {
			message: fmt.Sprintf("event with name %s already exists", name),
		}
	}

	event := NewStubDebuggingEvent(name)
	
	event.SetConn(m.JdwpConnection)	
	m.registeredEvents = append(m.registeredEvents, event)

	return event, nil
}

func (m *EventManager) GetEvent(name string) (*DebuggingEvent, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var event *DebuggingEvent
	var eventFound bool = false
	for _, foundEvent := range m.registeredEvents {
		if foundEvent.Name == name {
			event = foundEvent
			eventFound = true
		}
	}

	if !eventFound {
		log.Printf("unable to find event with name %s\n", name)
		return nil, JdwpDebuggingEventError {
			message: fmt.Sprintf("unable to find event with name %s", name),
		}
	}

	return event, nil
}

func (m *EventManager) GetAllEvents() ([]*DebuggingEvent, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	returnedEvents := append(m.registeredEvents, []*DebuggingEvent{}...)

	return returnedEvents, nil
}

func (m *EventManager) RunEvent(name string) error {
	m.mu.RLock()

	var event *DebuggingEvent
	var eventFound bool = false
	for _, foundEvent := range m.registeredEvents {
		if foundEvent.Name == name {
			event = foundEvent
			eventFound = true
		}
	}

	if !eventFound {
		log.Printf("unable to find event with name %s\n", name)
		return JdwpDebuggingEventError {
			message: fmt.Sprintf("unable to find event with name %s", name),
		}
	}

	m.mu.RUnlock()

	_, err := event.Run()
	return err
}

func (m *EventManager) CancelEvent(name string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var event *DebuggingEvent
	var eventFound bool = false
	for _, foundEvent := range m.registeredEvents {
		if foundEvent.Name == name {
			event = foundEvent
			eventFound = true
		}
	}

	if !eventFound {
		log.Printf("unable to find event with name %s\n", name)
		return JdwpDebuggingEventError {
			message: fmt.Sprintf("unable to find event with name %s", name),
		}
	}

	m.mu.RUnlock()
	
	return event.Cancel()
}

func (m *EventManager) DeregisterEvent(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var event *DebuggingEvent
	var eventIndex int = -1
	for i, foundEvent := range m.registeredEvents {
		if foundEvent.Name == name {
			event = foundEvent
			eventIndex = i
		}
	}

	if eventIndex < 0 {
		log.Printf("unable to find event with name %s\n", name)
		return JdwpDebuggingEventError {
			message: fmt.Sprintf("unable to find event with name %s", name),
		}
	}

	event.mu.Lock()
	
	if event.ctx != nil {
		return JdwpDebuggingEventError{
			message: fmt.Sprintf("event %s is running\n", name),
		}
	}

	m.registeredEvents = append(
		m.registeredEvents[:eventIndex],
		m.registeredEvents[(eventIndex + 1):]...,
	)

	return nil
}
