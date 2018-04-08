// +build gif

package main

import (
	"container/list"
	"time"
	"unsafe"
)

/*
#cgo CFLAGS: -Wno-deprecated-declarations -std=c99 -D_POSIX_C_SOURCE=200809L
#cgo LDFLAGS:-lgif
#include "gif.h"
*/
import "C"

const GIF_SUPPORT_ENABLED = true

type AnimatedFrame struct {
	qf C.QuantizedFrame
	ts time.Time
}

func quantizeFrame(frame unsafe.Pointer, width int, height int) C.QuantizedFrame {
	var qf C.QuantizedFrame
	C.QuantizeFrame(frame, C.int(width), C.int(height), &qf)

	return qf
}

func freeQuantizedFrame(qf C.QuantizedFrame) {
	C.FreeQuantizedFrame(&qf)
}

func encodeGif(qFrames list.List) (unsafe.Pointer, int) {
	qf := make([]C.QuantizedFrame, qFrames.Len())
	i := 0
	for f := qFrames.Front(); f != nil; f = f.Next() {
		q := f.Value.(AnimatedFrame)
		ts0 := q.ts
		ts1 := time.Now()
		if f.Next() != nil {
			ts1 = f.Next().Value.(AnimatedFrame).ts
		}

		qf[i] = q.qf
		qf[i].delay = C.int(ts1.Sub(ts0).Nanoseconds() / 10000000) //in 1/100 seconds
		i = i + 1
	}

	var gifData unsafe.Pointer
	size := C.WriteGif(&qf[0], C.int(len(qf)), &gifData)

	return gifData, int(size)
}
