// generated by github.com/vektah/dataloaden ; DO NOT EDIT

package graph

import (
	"sync"
	"time"

	"github.com/nycmonkey/gqlftw/model"
)

// PersonLoader batches and caches requests
type PersonLoader struct {
	// this method provides the data for the loader
	fetch func(keys []string) ([]*model.Person, []error)

	// how long to done before sending a batch
	wait time.Duration

	// this will limit the maximum number of keys to send in one batch, 0 = no limit
	maxBatch int

	// INTERNAL

	// lazily created cache
	cache map[string]*model.Person

	// the current batch. keys will continue to be collected until timeout is hit,
	// then everything will be sent to the fetch method and out to the listeners
	batch *personBatch

	// mutex to prevent races
	mu sync.Mutex
}

type personBatch struct {
	keys    []string
	data    []*model.Person
	error   []error
	closing bool
	done    chan struct{}
}

// Load a person by key, batching and caching will be applied automatically
func (l *PersonLoader) Load(key string) (*model.Person, error) {
	return l.LoadThunk(key)()
}

// LoadThunk returns a function that when called will block waiting for a person.
// This method should be used if you want one goroutine to make requests to many
// different data loaders without blocking until the thunk is called.
func (l *PersonLoader) LoadThunk(key string) func() (*model.Person, error) {
	l.mu.Lock()
	if it, ok := l.cache[key]; ok {
		l.mu.Unlock()
		return func() (*model.Person, error) {
			return it, nil
		}
	}
	if l.batch == nil {
		l.batch = &personBatch{done: make(chan struct{})}
	}
	batch := l.batch
	pos := batch.keyIndex(l, key)
	l.mu.Unlock()

	return func() (*model.Person, error) {
		<-batch.done

		var data *model.Person
		if pos < len(batch.data) {
			data = batch.data[pos]
		}

		var err error
		// its convenient to be able to return a single error for everything
		if len(batch.error) == 1 {
			err = batch.error[pos]
		} else if batch.error != nil {
			err = batch.error[pos]
		}

		if err == nil {
			l.mu.Lock()
			if l.cache == nil {
				l.cache = map[string]*model.Person{}
			}
			l.cache[key] = data
			l.mu.Unlock()
		}

		return data, err
	}
}

// LoadAll fetches many keys at once. It will be broken into appropriate sized
// sub batches depending on how the loader is configured
func (l *PersonLoader) LoadAll(keys []string) ([]*model.Person, []error) {
	results := make([]func() (*model.Person, error), len(keys))

	for i, key := range keys {
		results[i] = l.LoadThunk(key)
	}

	persons := make([]*model.Person, len(keys))
	errors := make([]error, len(keys))
	for i, thunk := range results {
		persons[i], errors[i] = thunk()
	}
	return persons, errors
}

// Prime the cache with the provided key and value. If the key already exists, no change is made.
// (To forcefully prime the cache, clear the key first with loader.clear(key).prime(key, value).)
func (l *PersonLoader) Prime(key string, value *model.Person) {
	l.mu.Lock()
	if _, found := l.cache[key]; !found {
		l.cache[key] = value
	}
	l.mu.Unlock()
}

// Clear the value at key from the cache, if it exists
func (l *PersonLoader) Clear(key string) {
	l.mu.Lock()
	delete(l.cache, key)
	l.mu.Unlock()
}

// keyIndex will return the location of the key in the batch, if its not found
// it will add the key to the batch
func (b *personBatch) keyIndex(l *PersonLoader, key string) int {
	for i, existingKey := range b.keys {
		if key == existingKey {
			return i
		}
	}

	pos := len(b.keys)
	b.keys = append(b.keys, key)
	if pos == 0 {
		go b.startTimer(l)
	}

	if l.maxBatch != 0 && pos >= l.maxBatch-1 {
		if !b.closing {
			b.closing = true
			l.batch = nil
			go b.end(l)
		}
	}

	return pos
}

func (b *personBatch) startTimer(l *PersonLoader) {
	time.Sleep(l.wait)
	l.mu.Lock()

	// we must have hit a batch limit and are already finalizing this batch
	if b.closing {
		l.mu.Unlock()
		return
	}

	l.batch = nil
	l.mu.Unlock()

	b.end(l)
}

func (b *personBatch) end(l *PersonLoader) {
	b.data, b.error = l.fetch(b.keys)
	close(b.done)
}