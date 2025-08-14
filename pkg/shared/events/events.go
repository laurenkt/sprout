package events

import (
	"context"
	"sync"
	"time"
)

// Event represents something that has happened in the system
type Event interface {
	Type() string
	Timestamp() time.Time
	Data() interface{}
}

// BaseEvent provides common event functionality
type BaseEvent struct {
	EventType string
	EventTime time.Time
	Payload   interface{}
}

func (e BaseEvent) Type() string {
	return e.EventType
}

func (e BaseEvent) Timestamp() time.Time {
	return e.EventTime
}

func (e BaseEvent) Data() interface{} {
	return e.Payload
}

// NewEvent creates a new base event
func NewEvent(eventType string, payload interface{}) Event {
	return BaseEvent{
		EventType: eventType,
		EventTime: time.Now(),
		Payload:   payload,
	}
}

// Handler defines a function that can process events
type Handler func(ctx context.Context, event Event) error

// Bus provides event publishing and subscription functionality
type Bus interface {
	// Publish sends an event to all subscribers
	Publish(ctx context.Context, event Event)
	
	// Subscribe registers a handler for events of a specific type
	Subscribe(eventType string, handler Handler) func()
	
	// SubscribeAll registers a handler for all events
	SubscribeAll(handler Handler) func()
	
	// Close shuts down the event bus
	Close() error
}

// InMemoryBus provides an in-memory event bus implementation
type InMemoryBus struct {
	handlers    map[string][]Handler
	allHandlers []Handler
	mu          sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
}

// NewInMemoryBus creates a new in-memory event bus
func NewInMemoryBus() *InMemoryBus {
	ctx, cancel := context.WithCancel(context.Background())
	return &InMemoryBus{
		handlers:    make(map[string][]Handler),
		allHandlers: make([]Handler, 0),
		ctx:         ctx,
		cancel:      cancel,
	}
}

// Publish sends an event to all subscribers
func (b *InMemoryBus) Publish(ctx context.Context, event Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	
	// Send to type-specific handlers
	if handlers, exists := b.handlers[event.Type()]; exists {
		for _, handler := range handlers {
			go b.handleEvent(ctx, handler, event)
		}
	}
	
	// Send to all-event handlers
	for _, handler := range b.allHandlers {
		go b.handleEvent(ctx, handler, event)
	}
}

// Subscribe registers a handler for events of a specific type
func (b *InMemoryBus) Subscribe(eventType string, handler Handler) func() {
	b.mu.Lock()
	defer b.mu.Unlock()
	
	b.handlers[eventType] = append(b.handlers[eventType], handler)
	
	// Return unsubscribe function
	return func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		
		handlers := b.handlers[eventType]
		for i, h := range handlers {
			// Compare function pointers (this is approximate)
			if &h == &handler {
				b.handlers[eventType] = append(handlers[:i], handlers[i+1:]...)
				break
			}
		}
		
		// Clean up empty handler lists
		if len(b.handlers[eventType]) == 0 {
			delete(b.handlers, eventType)
		}
	}
}

// SubscribeAll registers a handler for all events
func (b *InMemoryBus) SubscribeAll(handler Handler) func() {
	b.mu.Lock()
	defer b.mu.Unlock()
	
	b.allHandlers = append(b.allHandlers, handler)
	
	// Return unsubscribe function
	return func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		
		for i, h := range b.allHandlers {
			// Compare function pointers (this is approximate)
			if &h == &handler {
				b.allHandlers = append(b.allHandlers[:i], b.allHandlers[i+1:]...)
				break
			}
		}
	}
}

// Close shuts down the event bus
func (b *InMemoryBus) Close() error {
	b.cancel()
	return nil
}

// handleEvent processes an event with a handler, recovering from panics
func (b *InMemoryBus) handleEvent(ctx context.Context, handler Handler, event Event) {
	defer func() {
		if r := recover(); r != nil {
			// Log panic or handle it appropriately
			// For now, we'll just ignore it to prevent crashes
		}
	}()
	
	// Create a timeout context for event handling
	handleCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	
	_ = handler(handleCtx, event)
}

// Common event types
const (
	WorktreeCreated = "worktree.created"
	WorktreeDeleted = "worktree.deleted"
	IssueExpanded   = "issue.expanded"
	IssueSelected   = "issue.selected"
	SubtaskCreated  = "subtask.created"
	ConfigChanged   = "config.changed"
)