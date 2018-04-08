package main

import (
	"sync"
	"unsafe"
)

//wraps a c array in a refcounted thingee
type Frame struct {
	data          []byte
	backingData   unsafe.Pointer
	height, width int
	refcount      int
	lock          sync.Mutex
}

func newFrame(frameData unsafe.Pointer, len int, width int, height int) *Frame {
	frame := &Frame{}
	frame.backingData = frameData
	frame.data = (*[1 << 30]byte)(frameData)[:len:len]
	frame.width = width
	frame.height = height
	frame.refcount = 1

	return frame
}

func newFrameFromBytes(data []byte, width int, height int) *Frame {
	frame := &Frame{}
	frame.backingData = nil
	frame.data = data
	frame.width = width
	frame.height = height
	frame.refcount = 1

	return frame
}

func (frame *Frame) acquire() {
	frame.lock.Lock()
	frame.refcount += 1
	frame.lock.Unlock()
}

func (frame *Frame) release() {
	frame.lock.Lock()
	frame.refcount -= 1
	if frame.refcount == 0 && frame.backingData != nil {
		frame.data = nil
		cFree(frame.backingData)
	}
	frame.lock.Unlock()
}
