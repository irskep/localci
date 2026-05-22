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
	Type     string `json:"type"`
	Resource string `json:"resource"`
	Data     any    `json:"data,omitempty"`
	Offset   int64  `json:"offset,omitempty"`
	Text     string `json:"text,omitempty"`
	Message  string `json:"message,omitempty"`
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
	h.publish(APIEvent{Type: EventTypeAppend, Resource: resource, Offset: offset, Text: text})
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
			delete(h.subscribers, sub)
			close(sub.events)
		}
	}
}

type EventNotifier struct {
	Root string
	Hub  *EventHub
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
	_, _, _, attempt := n.entryResources(entry)
	if attempt == "" {
		return
	}
	resource := path.Join(attempt, "artifact", artifact)
	n.Hub.PublishAppend(resource, offset, text)
}

func (n EventNotifier) entryResources(entry QueueEntry) (string, string, string, string) {
	repoPath, err := RouteRepoPath(n.Root, entry.RepoDir)
	if err != nil {
		return "", "", "", ""
	}
	repo := path.Join("/api/repo", repoPath)
	commitPath, err := CommitRoutePath(n.Root, entry.RepoDir, entry.Commit)
	if err != nil {
		return repo, "", "", ""
	}
	taskPath, err := TaskRoutePath(n.Root, entry.RepoDir, entry.Commit, entry.TaskName)
	if err != nil {
		return repo, "/api" + commitPath, "", ""
	}
	attemptPath, err := AttemptRoutePath(n.Root, entry.RepoDir, entry.Commit, entry.TaskName, entry.Attempt)
	if err != nil {
		return repo, "/api" + commitPath, "/api" + taskPath, ""
	}
	commit := "/api" + commitPath
	task := "/api" + taskPath
	attempt := "/api" + attemptPath
	return repo, commit, task, attempt
}
