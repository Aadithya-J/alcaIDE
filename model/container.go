package model

import (
	"sync"
)

type ContainerInfo struct {
	ID string
	IsBusy bool
	mu sync.Mutex
}

// Method to safely check and acquire the container if free
func (c *ContainerInfo) TryAcquire() bool {
    c.mu.Lock()
    defer c.mu.Unlock()
    if c.IsBusy {
        return false
    }
    c.IsBusy = true
    return true 
}

// Method to safely release the container
func (c *ContainerInfo) Release() {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.IsBusy = false 
}