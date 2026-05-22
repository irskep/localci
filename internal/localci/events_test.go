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

func TestEventHubDropsSlowSubscriber(t *testing.T) {
	t.Parallel()

	hub := NewEventHub()
	events, unsubscribe := hub.Subscribe("/api/queue")
	defer unsubscribe()

	for range 32 {
		hub.PublishReplace("/api/queue")
	}

	received := 0
	for range events {
		received++
	}
	if received != 16 {
		t.Fatalf("received %d events, want buffered events before drop", received)
	}
}
