package store

import "log"

type EventStore struct {
	events       []Event
	eventsCh     chan Event
	registerCh   chan EventHandler
	unregisterCh chan EventHandler
	handlers     []EventHandler
}

func NewEventStore() *EventStore {
	var c EventStore

	return &c
}

func (store *EventStore) Events() []Event {
	return store.events
}

func (store *EventStore) Run() {
	store.eventsCh = make(chan Event)
	store.registerCh = make(chan EventHandler)
	store.unregisterCh = make(chan EventHandler)

	go func() {
		for {
			select {
			case e := <-store.eventsCh:
				e.Id = store.nextID(store.events)
				store.events = append(store.events, e)
				for _, s := range store.handlers {
					s.cb(store, e)
				}
			case reg := <-store.registerCh:
				store.handlers = append(store.handlers, reg)
			case unreg := <-store.unregisterCh:
				for i := range store.handlers {
					if unreg == store.handlers[i] {
						store.handlers = append(store.handlers[:i], store.handlers[i+1:]...)
						log.Println("event handler deregistered")
						break
					}
				}
			}
		}
	}()
}

func (store *EventStore) nextID(events []Event) int {
	if len(events) == 0 {
		return 0
	}
	return events[len(events)-1].Id + 1
}

func (store *EventStore) Persist(e Event) {
	go func() {
		store.eventsCh <- e
	}()
}

func (store *EventStore) Register(s EventHandler) {
	go func() {
		store.registerCh <- s
	}()
}

func (store *EventStore) Deregister(s EventHandler) {
	go func() {
		store.unregisterCh <- s
	}()
}
