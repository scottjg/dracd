// +build !gif

package main

import (
	"container/list"
	"time"
	"unsafe"
)

/*
#cgo CFLAGS: -DDISABLE_GIF=1
*/
import "C"

const GIF_SUPPORT_ENABLED = false

type AnimatedFrame struct {
	qf QF
	ts time.Time
}

type QF struct{}

func quantizeFrame(frame unsafe.Pointer, width int, height int) QF {
	return QF{}
}

func freeQuantizedFrame(qf QF) {

}

func encodeGif(qFrames list.List) (unsafe.Pointer, int) {
	return nil, 0
}
