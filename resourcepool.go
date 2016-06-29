package resourcepool

import (
	"container/list"
	"errors"
	"sync"
	"time"
)

const DefaultPoolSize = 32
const DefaultGetTimeoutSecond = 3

type ResourcePool struct {
	lock         sync.Mutex
	host         string
	port         string
	creationFunc ClientCreationFunc
	closeFunc    ClientCloseFunc
	maxSize      int
	getTimeout   int
	busyList     list.List
	idleList     chan interface{}
}

// ClientCreationFunc is the function used for creating new client.
type ClientCreationFunc func(host, port string) (interface{}, error)

// ClientCloseFunc is the function used for closing client.
type ClientCloseFunc func(interface{}) error

// AddServer adds a new server to the pool.
func NewResourcePool(host, port string, fnCreation ClientCreationFunc, fnClose ClientCloseFunc, maxSize, getTimeout int) (*ResourcePool, error) {
	pool := ResourcePool{
		maxSize:      maxSize,
		host:         host,
		port:         port,
		creationFunc: fnCreation,
		closeFunc:    fnClose,
		getTimeout:   getTimeout,
		idleList:     make(chan interface{}, maxSize),
	}
	return &pool, nil
}

// Get retrives a connection from the pool.
func (pool *ResourcePool) Get() (interface{}, error) {
	var res interface{}
	var err error
	if pool.Count() < pool.maxSize {
		res, err = pool.creationFunc(pool.host, pool.port)
	} else {
		if pool.getTimeout != 0 {
			select {
			case res = <-pool.idleList:
			case <-time.After(time.Second * time.Duration(pool.getTimeout)):
				res = nil
				err = errors.New("timed out")
			}
		} else {
			select {
			case res = <-pool.idleList:
			}
		}
	}
	if err == nil {
		pool.lock.Lock()
		defer pool.lock.Unlock()
		pool.busyList.PushBack(res)
	}
	return res, err
}

// Release puts the connection back to the pool.
func (pool *ResourcePool) Release(c interface{}) error {
	pool.lock.Lock()
	defer pool.lock.Unlock()
	element := pool.busyList.Front()
	for {
		if element == nil {
			return errors.New("the client isn't found in the pool, is it a managed client?")
		}
		if c == element.Value {
			pool.idleList <- c
			pool.busyList.Remove(element)
			return nil
		}
		element = element.Next()
	}
}

// Destroy disconnects all connectsions.
func (pool *ResourcePool) Destroy() {
	for res := range pool.idleList {
		pool.closeFunc(res)
	}
}

// Replace replaces existing connections to oldServer with connections to newServer.
func (pool *ResourcePool) Replace(oldHost, oldPort, newHost, newPort string) {
	pool.lock.Lock()
	defer pool.lock.Unlock()
}

// Count returns total number of connections in the pool.
func (pool *ResourcePool) Count() int {
	pool.lock.Lock()
	defer pool.lock.Unlock()
	return len(pool.idleList) + pool.busyList.Len()
}
