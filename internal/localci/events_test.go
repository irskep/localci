package localci

import "testing"

func TestEventHubPublishesToMatchingResourceOnly(t *testing.T) {
	t.Parallel()

	hub := NewEventHub()
	matching, unsubscribeMatching := hub.Subscribe("/api/queue")
	defer unsubscribeMatching()
	other, unsubscribeOther := hub.Subscribe("/api")
	defer unsubscribeOther()

	hub.PublishReplace("/api/queue")

	event := <-matching
	if event.Type != EventTypeReplace || event.Resource != "/api/queue" {
		t.Fatalf("event = %#v, want queue replacement", event)
	}

	select {
	case event := <-other:
		t.Fatalf("unrelated subscriber received event: %#v", event)
	default:
	}
}

func TestEventHubResyncsSlowSubscriberWithoutDisconnecting(t *testing.T) {
	t.Parallel()

	hub := NewEventHub()
	events, unsubscribe := hub.Subscribe("/api/queue")
	defer unsubscribe()

	for range 32 {
		hub.PublishReplace("/api/queue")
	}

	event := <-events
	if event.Type != EventTypeReplace {
		t.Fatalf("expected resync, got %#v", event)
	}
	if eventHubSubscriberCount(hub) != 1 {
		t.Fatal("slow subscriber was disconnected")
	}
	drainEvents(events)
	hub.PublishAppend("/api/queue", 0, "still connected")
	if event := <-events; event.Type != EventTypeAppend {
		t.Fatalf("event: %#v", event)
	}
}
