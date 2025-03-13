package events

var (
	listeners []func(Event)

	// event names here
)

type Event struct {
	Name    string
	Payload []interface{}
}

func Listen(fn func(Event)) {
	listeners = append(listeners, fn)
}

func New(name string, payload ...interface{}) Event {
	return Event{
		Name:    name,
		Payload: payload,
	}
}

func (e Event) Publish() {
	for _, fn := range listeners {
		fn(e)
	}
}
