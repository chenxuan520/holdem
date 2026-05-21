package match

type pendingEvent struct {
	EventType  string
	Visibility string
	Payload    any
}

func publicEvent(eventType string, payload any) pendingEvent {
	return pendingEvent{EventType: eventType, Visibility: "public", Payload: payload}
}

func replayOnlyEvent(eventType string, payload any) pendingEvent {
	return pendingEvent{EventType: eventType, Visibility: "replay_only", Payload: payload}
}
