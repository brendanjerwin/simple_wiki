package chatbuffer

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	// MaxMessagesPerPage is the maximum number of messages stored per page.
	MaxMessagesPerPage = 200

	// BufferInactivityTimeout is the duration after which an inactive buffer is reclaimed.
	BufferInactivityTimeout = 24 * time.Hour
)

var (
	// ErrMessageNotFound is returned when a message ID doesn't exist in the buffer.
	ErrMessageNotFound = errors.New("message not found")

	// ErrNoSubscribers is returned when trying to send a message but no channel subscribers are connected.
	ErrNoSubscribers = errors.New("no channel subscribers connected")
)

// Message represents a chat message stored in the buffer.
type Message struct {
	ID         string
	Sender     string // "user" or "assistant"
	Content    string
	Timestamp  time.Time
	Page       string
	Sequence   int64
	SenderName string
	ReplyToID  string
	Reactions  []Reaction
}

// Reaction represents an emoji reaction on a message.
type Reaction struct {
	Emoji   string
	Reactor string
}

// PermissionOption represents a single option in a permission request.
type PermissionOption struct {
	OptionID    string
	Label       string
	Description string
}

// PermissionRequestEvent represents a permission request sent to the browser.
type PermissionRequestEvent struct {
	Page        string
	RequestID   string
	Title       string
	Description string
	Options     []PermissionOption
}

// Event represents a chat event that can be streamed to subscribers.
type Event struct {
	Type              EventType
	Message           *Message
	Edit              *EditEvent
	Reaction          *ReactionEvent
	ToolCall          *ToolCallEvent
	PermissionRequest *PermissionRequestEvent
}

// EventType identifies the type of chat event.
type EventType int

const (
	EventTypeNewMessage EventType = iota
	EventTypeEdit
	EventTypeReaction
	EventTypeToolCall
	EventTypePermissionRequest
)

// EditEvent represents a message edit.
type EditEvent struct {
	MessageID  string
	NewContent string
	Timestamp  time.Time
	Streaming  bool // true for ACP streaming updates, false for user edits
}

// ToolCallEvent represents an agent tool invocation notification.
type ToolCallEvent struct {
	MessageID  string
	ToolCallID string
	Title      string
	Status     string // "pending", "running", "complete", "error"
}

// ReactionEvent represents a reaction added to a message.
type ReactionEvent struct {
	MessageID string
	Emoji     string
	Reactor   string
}

// pageBuffer stores messages for a single page.
type pageBuffer struct {
	messages       []*Message
	lastAccess     time.Time
	nextSequence   int64
	mu             sync.RWMutex
	eventListeners []chan Event
}

// Manager manages chat message buffers for all pages.
type Manager struct {
	buffers map[string]*pageBuffer
	mu      sync.RWMutex

	pageChannelSubscribers   map[string][]chan *Message // page → per-page MCP subscribers
	pageChannelSubscribersMu sync.RWMutex

	instanceRequests     map[string]time.Time // page → request timestamp
	instanceRequestChans []chan string         // pool daemon subscribers
	instanceMu           sync.RWMutex

	cancelFuncs   map[string][]chan struct{} // page → cancellation signal subscribers
	cancelFuncsMu sync.Mutex

	pendingPermissions   map[string]chan string // request_id → response channel
	pendingPermissionsMu sync.Mutex

	done chan struct{}
}

// NewManager creates a new chat buffer manager.
func NewManager() *Manager {
	m := &Manager{
		buffers:                make(map[string]*pageBuffer),
		pageChannelSubscribers: make(map[string][]chan *Message),
		instanceRequests:       make(map[string]time.Time),
		instanceRequestChans:   make([]chan string, 0),
		cancelFuncs:            make(map[string][]chan struct{}),
		pendingPermissions:     make(map[string]chan string),
		done:                   make(chan struct{}),
	}

	// Start background goroutine to reclaim stale buffers
	go m.reclaimStaleBuffers()

	return m
}

// Close shuts down the Manager, stopping the background reclaim goroutine.
func (m *Manager) Close() {
	close(m.done)
}

// reclaimStaleBuffers periodically removes buffers that haven't been accessed recently.
// Buffers with active listeners are never reclaimed to avoid breaking existing subscriptions.
func (m *Manager) reclaimStaleBuffers() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-m.done:
			return
		case <-ticker.C:
			m.mu.Lock()
			now := time.Now()
			for page, buf := range m.buffers {
				buf.mu.RLock()
				inactive := now.Sub(buf.lastAccess) > BufferInactivityTimeout
				hasActiveListeners := len(buf.eventListeners) > 0
				buf.mu.RUnlock()

				// Only reclaim inactive buffers that have no active subscribers
				if inactive && !hasActiveListeners {
					delete(m.buffers, page)
				}
			}
			m.mu.Unlock()

			// Clear stale instance requests (older than 2 minutes)
			m.instanceMu.Lock()
			for page, ts := range m.instanceRequests {
				if now.Sub(ts) > 2*time.Minute {
					delete(m.instanceRequests, page)
				}
			}
			m.instanceMu.Unlock()
		}
	}
}

// getOrCreateBuffer returns the buffer for a page, creating it if necessary.
func (m *Manager) getOrCreateBuffer(page string) *pageBuffer {
	m.mu.Lock()
	defer m.mu.Unlock()

	buf, exists := m.buffers[page]
	if !exists {
		buf = &pageBuffer{
			messages:       make([]*Message, 0, MaxMessagesPerPage),
			lastAccess:     time.Now(),
			nextSequence:   1,
			eventListeners: make([]chan Event, 0),
		}
		m.buffers[page] = buf
	}

	buf.mu.Lock()
	buf.lastAccess = time.Now()
	buf.mu.Unlock()

	return buf
}

// AddUserMessage adds a user message to the buffer and notifies subscribers.
// Returns ErrNoSubscribers if no page-level channel subscribers or pool daemon are connected;
// in that case the message is NOT stored and page subscribers are NOT notified.
func (m *Manager) AddUserMessage(page, content, senderName string) (string, error) {
	// Check for channel subscribers BEFORE writing to the buffer so that if
	// ErrNoSubscribers is returned, the message was never stored and page
	// subscribers were never notified.
	if !m.HasPageChannelSubscriber(page) && !m.HasInstanceRequestSubscribers() {
		return "", ErrNoSubscribers
	}

	buf := m.getOrCreateBuffer(page)
	buf.mu.Lock()

	messageID := uuid.New().String()
	sequence := buf.nextSequence
	buf.nextSequence++

	msg := &Message{
		ID:         messageID,
		Sender:     "user",
		Content:    content,
		Timestamp:  time.Now(),
		Page:       page,
		Sequence:   sequence,
		SenderName: senderName,
		Reactions:  make([]Reaction, 0),
	}

	buf.appendToRingBuffer(msg)

	// Capture a copy of the message before releasing the lock.
	// Sending copies prevents callers of EditMessage/AddReaction from racing
	// with consumers reading the message fields.
	msgCopy := *msg
	buf.unlockAndNotify(Event{
		Type:    EventTypeNewMessage,
		Message: &msgCopy,
	})

	// Publish to page-specific channel subscribers (wiki-cli mcp --page / ACP pool).
	m.pageChannelSubscribersMu.RLock()
	if subs, ok := m.pageChannelSubscribers[page]; ok {
		msgCopy2 := *msg
		for _, subscriber := range subs {
			select {
			case subscriber <- &msgCopy2:
			default:
				// Don't block if subscriber is slow
			}
		}
	}
	m.pageChannelSubscribersMu.RUnlock()

	return messageID, nil
}

// AddAssistantMessage adds an assistant message to the buffer and notifies page subscribers.
func (m *Manager) AddAssistantMessage(page, content, replyToID string) (string, error) {
	buf := m.getOrCreateBuffer(page)
	buf.mu.Lock()

	messageID := uuid.New().String()
	sequence := buf.nextSequence
	buf.nextSequence++

	msg := &Message{
		ID:         messageID,
		Sender:     "assistant",
		Content:    content,
		Timestamp:  time.Now(),
		Page:       page,
		Sequence:   sequence,
		SenderName: "",
		ReplyToID:  replyToID,
		Reactions:  make([]Reaction, 0),
	}

	buf.appendToRingBuffer(msg)

	// Capture a copy of the message before releasing the lock.
	// Sending copies prevents callers of EditMessage/AddReaction from racing
	// with consumers reading the message fields.
	msgCopy := *msg
	buf.unlockAndNotify(Event{
		Type:    EventTypeNewMessage,
		Message: &msgCopy,
	})

	return messageID, nil
}

// notifyEventListeners sends an event to all listeners without blocking.
func notifyEventListeners(listeners []chan Event, event Event) {
	for _, listener := range listeners {
		select {
		case listener <- event:
		default:
			// Don't block if listener is slow
		}
	}
}

// unlockAndNotify copies the event listeners, releases buf.mu, then notifies them.
// MUST be called while holding buf.mu (write lock).
func (buf *pageBuffer) unlockAndNotify(event Event) {
	listeners := make([]chan Event, len(buf.eventListeners))
	copy(listeners, buf.eventListeners)
	buf.mu.Unlock()
	notifyEventListeners(listeners, event)
}

// makeUnsubscribeFn returns a function that removes eventChan from buf.eventListeners and closes it.
func (buf *pageBuffer) makeUnsubscribeFn(eventChan chan Event) func() {
	return func() {
		buf.mu.Lock()
		defer buf.mu.Unlock()

		for i, listener := range buf.eventListeners {
			if listener == eventChan {
				buf.eventListeners = append(buf.eventListeners[:i], buf.eventListeners[i+1:]...)
				close(eventChan)
				break
			}
		}
	}
}

// appendToRingBuffer adds msg to the buffer, evicting the oldest message if full.
func (buf *pageBuffer) appendToRingBuffer(msg *Message) {
	buf.messages = append(buf.messages, msg)
	if len(buf.messages) > MaxMessagesPerPage {
		buf.messages[0] = nil          // Allow GC of evicted message
		buf.messages = buf.messages[1:] // Evict oldest
	}
}

// clone returns a deep copy of msg including its Reactions slice.
func (msg *Message) clone() *Message {
	msgCopy := *msg
	msgCopy.Reactions = make([]Reaction, len(msg.Reactions))
	copy(msgCopy.Reactions, msg.Reactions)
	return &msgCopy
}

// hasReaction reports whether msg already has the given emoji reaction from reactor.
func hasReaction(msg *Message, emoji, reactor string) bool {
	for _, r := range msg.Reactions {
		if r.Emoji == emoji && r.Reactor == reactor {
			return true
		}
	}
	return false
}

// EditMessage updates the content of an existing message.
// If streaming is true, the edit event carries a streaming flag so the frontend
// can suppress the "(edited)" indicator for ACP streaming updates.
func (m *Manager) EditMessage(messageID, newContent string, streaming bool) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Find the message across all buffers
	for _, buf := range m.buffers {
		buf.mu.Lock()
		for _, msg := range buf.messages {
			if msg.ID != messageID {
				continue
			}

			msg.Content = newContent
			buf.lastAccess = time.Now() // Update lastAccess to prevent premature reclamation

			buf.unlockAndNotify(Event{
				Type: EventTypeEdit,
				Edit: &EditEvent{
					MessageID:  messageID,
					NewContent: newContent,
					Timestamp:  time.Now(),
					Streaming:  streaming,
				},
			})
			return nil
		}
		buf.mu.Unlock()
	}

	return ErrMessageNotFound
}

// AddReaction adds a reaction to a message.
func (m *Manager) AddReaction(messageID, emoji, reactor string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Find the message across all buffers
	for _, buf := range m.buffers {
		buf.mu.Lock()
		for _, msg := range buf.messages {
			if msg.ID != messageID {
				continue
			}

			if !hasReaction(msg, emoji, reactor) {
				msg.Reactions = append(msg.Reactions, Reaction{
					Emoji:   emoji,
					Reactor: reactor,
				})
			}

			buf.lastAccess = time.Now() // Update lastAccess to prevent premature reclamation

			buf.unlockAndNotify(Event{
				Type: EventTypeReaction,
				Reaction: &ReactionEvent{
					MessageID: messageID,
					Emoji:     emoji,
					Reactor:   reactor,
				},
			})
			return nil
		}
		buf.mu.Unlock()
	}

	return ErrMessageNotFound
}

// NotifyToolCall sends a tool call notification to page subscribers.
// This is ephemeral — not stored in the buffer, only streamed to active subscribers.
func (m *Manager) NotifyToolCall(page, messageID, toolCallID, title, toolStatus string) {
	buf := m.getOrCreateBuffer(page)
	buf.mu.Lock()
	buf.lastAccess = time.Now()

	buf.unlockAndNotify(Event{
		Type: EventTypeToolCall,
		ToolCall: &ToolCallEvent{
			MessageID:  messageID,
			ToolCallID: toolCallID,
			Title:      title,
			Status:     toolStatus,
		},
	})
}

// GetMessages returns all messages for a page.
func (m *Manager) GetMessages(page string) []*Message {
	buf := m.getOrCreateBuffer(page)
	buf.mu.RLock()
	defer buf.mu.RUnlock()

	// Return copies to prevent race conditions
	messages := make([]*Message, len(buf.messages))
	for i, msg := range buf.messages {
		messages[i] = msg.clone()
	}

	return messages
}

// SubscribeToPage subscribes to chat events for a specific page.
// Returns a channel that receives events and an unsubscribe function.
func (m *Manager) SubscribeToPage(page string) (<-chan Event, func()) {
	buf := m.getOrCreateBuffer(page)
	buf.mu.Lock()
	defer buf.mu.Unlock()

	eventChan := make(chan Event, 100) // Buffer events
	buf.eventListeners = append(buf.eventListeners, eventChan)

	return eventChan, buf.makeUnsubscribeFn(eventChan)
}

// SubscribeToPageWithReplay atomically subscribes to a page and returns existing messages
// to prevent race conditions between GetMessages and SubscribeToPage.
// Returns existing messages, event channel, and unsubscribe function.
func (m *Manager) SubscribeToPageWithReplay(page string) ([]*Message, <-chan Event, func()) {
	buf := m.getOrCreateBuffer(page)
	buf.mu.Lock()
	defer buf.mu.Unlock()

	// Create subscription
	eventChan := make(chan Event, 100) // Buffer events
	buf.eventListeners = append(buf.eventListeners, eventChan)

	// Copy existing messages under the same lock
	messages := make([]*Message, len(buf.messages))
	for i, msg := range buf.messages {
		messages[i] = msg.clone()
	}

	return messages, eventChan, buf.makeUnsubscribeFn(eventChan)
}

// GetMessageByID retrieves a message by its ID across all pages.
func (m *Manager) GetMessageByID(messageID string) (*Message, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, buf := range m.buffers {
		buf.mu.RLock()
		for _, msg := range buf.messages {
			if msg.ID == messageID {
				msgCopy := msg.clone()
				buf.mu.RUnlock()
				return msgCopy, nil
			}
		}
		buf.mu.RUnlock()
	}

	return nil, fmt.Errorf("message %s: %w", messageID, ErrMessageNotFound)
}

// SubscribeToPageChannel subscribes to user messages for a specific page only.
// Used by wiki-cli mcp --page to receive messages for a single page's Claude instance.
// Returns a channel that receives messages and an unsubscribe function.
func (m *Manager) SubscribeToPageChannel(page string) (<-chan *Message, func()) {
	m.pageChannelSubscribersMu.Lock()
	defer m.pageChannelSubscribersMu.Unlock()

	msgChan := make(chan *Message, 100)
	m.pageChannelSubscribers[page] = append(m.pageChannelSubscribers[page], msgChan)

	unsubscribe := func() {
		m.pageChannelSubscribersMu.Lock()
		defer m.pageChannelSubscribersMu.Unlock()

		subs := m.pageChannelSubscribers[page]
		for i, subscriber := range subs {
			if subscriber == msgChan {
				m.pageChannelSubscribers[page] = append(subs[:i], subs[i+1:]...)
				close(msgChan)
				break
			}
		}
		if len(m.pageChannelSubscribers[page]) == 0 {
			delete(m.pageChannelSubscribers, page)
		}
	}

	return msgChan, unsubscribe
}

// SubscribeToPageChannelWithReplay atomically subscribes to a page channel and returns
// existing messages, preventing the first message from being lost between spawn and subscribe.
func (m *Manager) SubscribeToPageChannelWithReplay(page string) ([]*Message, <-chan *Message, func()) {
	buf := m.getOrCreateBuffer(page)
	buf.mu.Lock()

	// Create page channel subscription
	m.pageChannelSubscribersMu.Lock()
	msgChan := make(chan *Message, 100)
	m.pageChannelSubscribers[page] = append(m.pageChannelSubscribers[page], msgChan)
	m.pageChannelSubscribersMu.Unlock()

	// Copy existing messages under the buffer lock
	messages := make([]*Message, len(buf.messages))
	for i, msg := range buf.messages {
		messages[i] = msg.clone()
	}
	buf.mu.Unlock()

	unsubscribe := func() {
		m.pageChannelSubscribersMu.Lock()
		defer m.pageChannelSubscribersMu.Unlock()
		subs := m.pageChannelSubscribers[page]
		for i, subscriber := range subs {
			if subscriber == msgChan {
				m.pageChannelSubscribers[page] = append(subs[:i], subs[i+1:]...)
				close(msgChan)
				break
			}
		}
		if len(m.pageChannelSubscribers[page]) == 0 {
			delete(m.pageChannelSubscribers, page)
		}
	}

	return messages, msgChan, unsubscribe
}

// HasPageChannelSubscriber returns true if a per-page channel subscriber exists for the given page.
func (m *Manager) HasPageChannelSubscriber(page string) bool {
	m.pageChannelSubscribersMu.RLock()
	defer m.pageChannelSubscribersMu.RUnlock()
	return len(m.pageChannelSubscribers[page]) > 0
}

// RequestInstance records that a page needs a Claude instance and notifies pool daemon subscribers.
// Deduplicates: if the page already has a subscriber or was recently requested, this is a no-op.
func (m *Manager) RequestInstance(page string) {
	// Don't request if page already has a per-page subscriber
	if m.HasPageChannelSubscriber(page) {
		return
	}

	m.instanceMu.Lock()
	defer m.instanceMu.Unlock()

	// Deduplicate: don't re-request if already requested within the last 2 minutes
	if ts, ok := m.instanceRequests[page]; ok && time.Since(ts) < 2*time.Minute {
		return
	}

	m.instanceRequests[page] = time.Now()

	// Notify exactly one pool daemon subscriber to prevent duplicate spawns.
	// Try each subscriber in order; use the first one that accepts.
	for _, ch := range m.instanceRequestChans {
		select {
		case ch <- page:
			return
		default:
			// Subscriber is full, try next
		}
	}
}

// SubscribeToInstanceRequests subscribes to instance request notifications.
// Used by the wiki-cli pool daemon to know when to spawn Claude instances.
// Returns a channel that receives page names and an unsubscribe function.
func (m *Manager) SubscribeToInstanceRequests() (<-chan string, func()) {
	m.instanceMu.Lock()
	defer m.instanceMu.Unlock()

	ch := make(chan string, 100)
	m.instanceRequestChans = append(m.instanceRequestChans, ch)

	unsubscribe := func() {
		m.instanceMu.Lock()
		defer m.instanceMu.Unlock()

		for i, subscriber := range m.instanceRequestChans {
			if subscriber == ch {
				m.instanceRequestChans = append(m.instanceRequestChans[:i], m.instanceRequestChans[i+1:]...)
				close(ch)
				break
			}
		}
	}

	return ch, unsubscribe
}

// HasInstanceRequestSubscribers returns true if any pool daemon is subscribed to instance requests.
func (m *Manager) HasInstanceRequestSubscribers() bool {
	m.instanceMu.RLock()
	defer m.instanceMu.RUnlock()
	return len(m.instanceRequestChans) > 0
}

// CancelPage signals cancellation for an in-progress agent prompt on the given page.
// Returns true if any cancellation subscribers were notified.
func (m *Manager) CancelPage(page string) bool {
	m.cancelFuncsMu.Lock()
	defer m.cancelFuncsMu.Unlock()

	chans, ok := m.cancelFuncs[page]
	if !ok || len(chans) == 0 {
		return false
	}

	// Signal all subscribers, then remove them (one-shot)
	for _, ch := range chans {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
	delete(m.cancelFuncs, page)
	return true
}

// SubscribeToCancellation returns a channel that receives a signal when CancelPage is called
// for the given page. The returned unsubscribe function removes the subscription.
func (m *Manager) SubscribeToCancellation(page string) (<-chan struct{}, func()) {
	m.cancelFuncsMu.Lock()
	defer m.cancelFuncsMu.Unlock()

	ch := make(chan struct{}, 1)
	m.cancelFuncs[page] = append(m.cancelFuncs[page], ch)

	unsubscribe := func() {
		m.cancelFuncsMu.Lock()
		defer m.cancelFuncsMu.Unlock()

		chans := m.cancelFuncs[page]
		for i, c := range chans {
			if c == ch {
				m.cancelFuncs[page] = append(chans[:i], chans[i+1:]...)
				break
			}
		}
		if len(m.cancelFuncs[page]) == 0 {
			delete(m.cancelFuncs, page)
		}
	}

	return ch, unsubscribe
}

// IsInstanceRequested returns true if a Claude instance has been requested for the given page
// within the last 2 minutes and has not yet connected (no page channel subscriber exists).
func (m *Manager) IsInstanceRequested(page string) bool {
	if m.HasPageChannelSubscriber(page) {
		return false
	}

	m.instanceMu.RLock()
	defer m.instanceMu.RUnlock()
	ts, ok := m.instanceRequests[page]
	return ok && time.Since(ts) < 2*time.Minute
}

// RequestPermission emits a permission request event to page subscribers and blocks
// until a response arrives via RespondToPermission. Returns the selected option ID.
func (m *Manager) RequestPermission(page, requestID, title, description string, options []PermissionOption) string {
	// Create a response channel and register it
	responseChan := make(chan string, 1)

	m.pendingPermissionsMu.Lock()
	m.pendingPermissions[requestID] = responseChan
	m.pendingPermissionsMu.Unlock()

	// Emit the permission request event to page subscribers
	m.EmitPermissionRequest(page, &PermissionRequestEvent{
		Page:        page,
		RequestID:   requestID,
		Title:       title,
		Description: description,
		Options:     options,
	})

	// Block until a response arrives
	selectedOptionID := <-responseChan

	// Clean up
	m.pendingPermissionsMu.Lock()
	delete(m.pendingPermissions, requestID)
	m.pendingPermissionsMu.Unlock()

	return selectedOptionID
}

// EmitPermissionRequest sends a permission request event to all subscribers of the given page.
func (m *Manager) EmitPermissionRequest(page string, event *PermissionRequestEvent) {
	buf := m.getOrCreateBuffer(page)
	buf.mu.Lock()
	buf.lastAccess = time.Now()

	buf.unlockAndNotify(Event{
		Type:              EventTypePermissionRequest,
		PermissionRequest: event,
	})
}

// RespondToPermission delivers a permission response to the waiting RequestPermission call.
func (m *Manager) RespondToPermission(requestID, selectedOptionID string) {
	m.pendingPermissionsMu.Lock()
	responseChan, ok := m.pendingPermissions[requestID]
	m.pendingPermissionsMu.Unlock()

	if !ok {
		return
	}

	select {
	case responseChan <- selectedOptionID:
	default:
		// Channel already has a response or was closed
	}
}
