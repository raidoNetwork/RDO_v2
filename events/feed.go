package events

import (
	"github.com/sirupsen/logrus"
	"reflect"
	"sync"
)

var log = logrus.WithField("prefix", "events")

type Emitter interface{
	Send(interface{}) int
}

type Subscription interface{
	Unsubscribe()
}

type eventSubscription struct{
	bus *Bus
	ch reflect.Value
}

func (es *eventSubscription) Unsubscribe() {
	es.bus.unsubscribe(es.ch)
}

type Bus struct{
	once sync.Once
	cases []reflect.SelectCase
	lock sync.Mutex
}

func (b *Bus) init(){
	b.cases = make([]reflect.SelectCase, 0)
}

func (b *Bus) Subscribe(ch interface{}) Subscription {
	b.once.Do(b.init)

	b.lock.Lock()
	defer b.lock.Unlock()

	cval := reflect.ValueOf(ch)
	esub := &eventSubscription{ch: cval, bus: b}
	b.cases = append(b.cases, reflect.SelectCase{Dir: reflect.SelectSend, Chan: cval})

	return esub
}

func (b *Bus) unsubscribe(ch reflect.Value){
	b.once.Do(b.init)

	b.lock.Lock()
	defer b.lock.Unlock()

	for i, c := range b.cases {
		if c.Chan == ch {
			b.deleteCase(i)
		}
	}
}

func (b *Bus) Send(data interface{}) int {
	b.once.Do(b.init)

	b.lock.Lock()
	defer b.lock.Unlock()

	cases := b.cases

	dval := reflect.ValueOf(data)
	sent := 0

	// setup value for all send cases
	for i, _ := range cases {
		cases[i].Send = dval
	}

	for {
		// try to send data
		for i := 0;i < len(cases);i++ {
			if cases[i].Chan.TrySend(dval) {
				cases = deleteItem(cases, i)
				sent++
				i--
			} else{
				log.Printf("Can't send %v", cases[i].Chan.Type().String())
			}
		}

		if 0 == len(cases){
			break
		}

		// Select on all the receivers, waiting for them to unblock.
		chosen, _, _ := reflect.Select(cases)
		cases = deleteItem(cases, chosen)
		sent++
	}

	// reset value of all send cases
	for i, _ := range b.cases {
		b.cases[i].Send = reflect.Value{}
	}

	return sent
}

func (b *Bus) deleteCase(index int){
	b.cases = deleteItem(b.cases, index)
}

func deleteItem(cases []reflect.SelectCase, index int) []reflect.SelectCase {
	last := len(cases) - 1
	cases[index], cases[last] = cases[last], cases[index]
	return cases[:last]
}