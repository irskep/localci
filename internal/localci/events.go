package localci

import (
	"path"
	"sync"
)

const (
	EventTypeSnapshot = "snapshot"
	EventTypeReplace  = "replace"
	EventTypeAppend   = "append"
	EventTypeRemove   = "remove"
	EventTypeError    = "error"
)

type APIEvent struct {
	Type      string `json:"type"`
	Resource  string `json:"resource"`
	Data      any    `json:"data,omitempty"`
	Offset    int64  `json:"offset"`
	EndOffset int64  `json:"end_offset"`
	Text      string `json:"text,omitempty"`
	Message   string `json:"message,omitempty"`
}

type EventHub struct {
	mu          sync.Mutex
	subscribers map[*eventSubscription]struct{}
}

type eventSubscription struct {
	resource string
	events   chan APIEvent
}

func NewEventHub() *EventHub {
	return &EventHub{
		subscribers: map[*eventSubscription]struct{}{},
	}
}

func (h *EventHub) Subscribe(resource string) (<-chan APIEvent, func()) {
	if h == nil {
		events := make(chan APIEvent)
		close(events)
		return events, func() {}
	}

	sub := &eventSubscription{
		resource: resource,
		events:   make(chan APIEvent, 16),
	}

	h.mu.Lock()
	h.subscribers[sub] = struct{}{}
	h.mu.Unlock()

	unsubscribe := func() {
		h.mu.Lock()
		if _, ok := h.subscribers[sub]; ok {
			delete(h.subscribers, sub)
			close(sub.events)
		}
		h.mu.Unlock()
	}
	return sub.events, unsubscribe
}

func (h *EventHub) PublishReplace(resource string) {
	h.publish(APIEvent{Type: EventTypeReplace, Resource: resource})
}

func (h *EventHub) PublishAppend(resource string, offset int64, text string) {
	h.publish(APIEvent{Type: EventTypeAppend, Resource: resource, Offset: offset, EndOffset: offset + int64(len(text)), Text: text})
}

func (h *EventHub) publish(event APIEvent) {
	if h == nil {
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	for sub := range h.subscribers {
		if sub.resource != event.Resource {
			continue
		}
		select {
		case sub.events <- event:
		default:
			// A snapshot subsumes pending appends and replacements. Keep the
			// connection instead of making a busy client reconnect repeatedly.
			drainEvents(sub.events)
			sub.events <- APIEvent{Type: EventTypeReplace, Resource: sub.resource}
		}
	}
}

type EventNotifier struct {
	Hub *EventHub
}

func (n EventNotifier) QueueChanged() {
	if n.Hub == nil {
		return
	}
	n.Hub.PublishReplace("/api")
	n.Hub.PublishReplace("/api/queue")
}

func (n EventNotifier) EntryChanged(entry QueueEntry) {
	if n.Hub == nil {
		return
	}

	n.QueueChanged()
	repo, commit, task, attempt := n.entryResources(entry)
	for _, resource := range []string{repo, commit, task, attempt} {
		if resource != "" {
			n.Hub.PublishReplace(resource)
		}
	}
}

func (n EventNotifier) ArtifactAppended(entry QueueEntry, artifact string, offset int64, text string) {
	if n.Hub == nil {
		return
	}
	_, _, task, attempt := n.entryResources(entry)
	if attempt == "" {
		return
	}
	resource := path.Join(attempt, "artifact", artifact)
	n.Hub.PublishAppend(resource, offset, text)
	if artifact == combinedLogName {
		if task != "" {
			n.Hub.PublishAppend(task, offset, text)
		}
		n.Hub.PublishAppend(attempt, offset, text)
	}
}

func (n EventNotifier) entryResources(entry QueueEntry) (string, string, string, string) {
	repoPath, err := RouteRepoPath(entry.RepoDir)
	if err != nil {
		return "", "", "", ""
	}
	repo := path.Join("/api/repo", repoPath)
	commitPath, err := CommitRoutePath(entry.RepoDir, entry.Commit)
	if err != nil {
		return repo, "", "", ""
	}
	taskPath, err := TaskRoutePath(entry.RepoDir, entry.Commit, entry.TaskName)
	if err != nil {
		return repo, "/api" + commitPath, "", ""
	}
	attemptPath, err := AttemptRoutePath(entry.RepoDir, entry.Commit, entry.TaskName, entry.Attempt)
	if err != nil {
		return repo, "/api" + commitPath, "/api" + taskPath, ""
	}
	commit := "/api" + commitPath
	task := "/api" + taskPath
	attempt := "/api" + attemptPath
	return repo, commit, task, attempt
}

func drainEvents(events <-chan APIEvent) {
	for range cap(events) {
		select {
		case _, ok := <-events:
			if !ok {
				return
			}
		default:
			return
		}
	}
}
