package pubsub

import (
	"sync"

	"github.com/bamdadam/backend/graph/model"
)

type ElementPubSub struct {
	mu   sync.RWMutex
	subs map[string]map[chan *model.Element]struct{}
}

func NewElementPubSub() *ElementPubSub {
	return &ElementPubSub{
		subs: make(map[string]map[chan *model.Element]struct{}),
	}
}

func (p *ElementPubSub) Subscribe(uri string) chan *model.Element {
	p.mu.Lock()
	defer p.mu.Unlock()

	ch := make(chan *model.Element, 1)
	if p.subs[uri] == nil {
		p.subs[uri] = make(map[chan *model.Element]struct{})
	}
	p.subs[uri][ch] = struct{}{}
	return ch
}

func (p *ElementPubSub) Unsubscribe(uri string, ch chan *model.Element) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if subs, ok := p.subs[uri]; ok {
		delete(subs, ch)
		if len(subs) == 0 {
			delete(p.subs, uri)
		}
	}
	close(ch)
}

func (p *ElementPubSub) Publish(elem *model.Element) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	for ch := range p.subs[elem.URI] {
		select {
		case ch <- elem:
		default:
		}
	}
}