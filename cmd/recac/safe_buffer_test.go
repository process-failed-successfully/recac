package main

import (
	"bytes"
	"sync"
)

type SafeBuffer struct {
	b bytes.Buffer
	m sync.Mutex
}

func (b *SafeBuffer) Read(p []byte) (n int, err error) {
	b.m.Lock()
	defer b.m.Unlock()
	return b.b.Read(p)
}

func (b *SafeBuffer) Write(p []byte) (n int, err error) {
	b.m.Lock()
	defer b.m.Unlock()
	return b.b.Write(p)
}

func (b *SafeBuffer) String() string {
	b.m.Lock()
	defer b.m.Unlock()
	return b.b.String()
}
