package events

var (
	listeners map[string]func(Event)

	ServerCreated = "server.created"
	ServerDeleted = "server.deleted"

	ServerInstallStarted  = "server.start_install"
	ServerInstallFinished = "server.finish_install"

	ServerLog = "server.log"
)

type Event struct {
	Name    string
	Payload interface{}
}

func Listen(id string, fn func(Event)) func() {
	listeners[id] = fn
	return func() {
		Unlisten(id)
	}
}

func Unlisten(id string) {
	delete(listeners, id)
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
