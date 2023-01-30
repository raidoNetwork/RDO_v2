package events

import (
	"reflect"
	"sync"

	"github.com/sirupsen/logrus"
)

var log = logrus.WithField("prefix", "events")

type Feed interface {
	Send(interface{}) int
	Subscribe(interface{}) Subscription
}

type Subscription interface {
	Unsubscribe()
}

type eventSubscription struct {
	bus *Bus
	ch  reflect.Value
}

func (es *eventSubscription) Unsubscribe() {
	es.bus.unsubscribe(es.ch)
}

type Bus struct {
	once  sync.Once
	cases cases
	lock  sync.Mutex
}

func (b *Bus) init() {
	b.cases = make(cases, 0)
}

func (b *Bus) Subscribe(ch interface{}) Subscription {
	b.once.Do(b.init)

	b.lock.Lock()
	defer b.lock.Unlock()

	cval := reflect.ValueOf(ch)
	ctype := cval.Type()

	if ctype.Kind() != reflect.Chan || ctype.ChanDir() == reflect.SendDir {
		panic("Bad events feed subscriber")
	}

	esub := &eventSubscription{ch: cval, bus: b}
	b.cases = append(b.cases, reflect.SelectCase{Dir: reflect.SelectSend, Chan: cval})

	return esub
}

func (b *Bus) unsubscribe(ch reflect.Value) {
	b.once.Do(b.init)

	b.lock.Lock()
	defer b.lock.Unlock()

	for i, c := range b.cases {
		if c.Chan == ch {
			b.cases = b.cases.delete(i)
		}
	}
}

func (b *Bus) Send(data interface{}) int {
	b.once.Do(b.init)

	b.lock.Lock()
	sendCases := b.cases

	dval := reflect.ValueOf(data)
	sent := 0

	// setup value for all send cases
	for i := 0; i < len(sendCases); i++ {
		sendCases[i].Send = dval
	}
	b.lock.Unlock()

	for {
		// try to send data
		for i := 0; i < len(sendCases); i++ {
			if sendCases[i].Chan.TrySend(dval) {
				sendCases = sendCases.delete(i)
				sent++
				i--
			}
		}

		if 0 == len(sendCases) {
			break
		}

		log.Errorf("Can't send event %v. Waiting subscribers %d", sendCases[0].Chan.Type().String(), len(sendCases))

		// Select on all the receivers, waiting for them to unblock.
		chosen, _, _ := reflect.Select(sendCases)
		sendCases = sendCases.delete(chosen)
		sent++
	}

	// reset value of all send cases
	for i := range b.cases {
		b.lock.Lock()
		b.cases[i].Send = reflect.Value{}
		b.lock.Unlock()
	}

	return sent
}

type cases []reflect.SelectCase

func (c cases) delete(i int) cases {
	if i < 0 || i >= len(c) {
		return c
	}

	last := len(c) - 1
	if i != last {
		c[i], c[last] = c[last], c[i]
	}

	return c[:last]
}

func (c cases) find(channel interface{}) int {
	for i, cas := range c {
		if cas.Chan.Interface() == channel {
			return i
		}
	}
	return -1
}
