package main

import (
	"container/list"
	"sync"
	//"crypto/tls"
	"log"
	"net"
	"time"
	"unsafe"
)

/*
#cgo CFLAGS: -Wno-deprecated-declarations -std=c99 -D_POSIX_C_SOURCE=200809L
#cgo LDFLAGS: -lpng
#include "goglue.h"
#include "protocol.h"
*/
import "C"

type DracCtx struct {
	ctrlSocket      net.Conn
	videoSocket     net.Conn
	ctx             *C.client_ctx
	vncClients      list.List
	host            string
	dracType        DracType
	sentFirstFrame  bool
	firstFrameEvent chan int
	startTime       time.Time

	otherClientCount int
	clientEvent      chan int

	lastPngFrame      *Frame
	animatedFrameList list.List
}

//golang panics if we pass pointers of go objects into C code, in anticipation
//of implementing GC primitives that might move golang objects in memory. so,
//we keep this table of go objects with ids that we can pass around instead
//of raw pointers
var dracCtxRefs map[int]*DracCtx = map[int]*DracCtx{}
var dracCtxRefsNextID int
var dracCtxRefsLock sync.Mutex

func AllocDracClientCtx(dracCtx *DracCtx) {
	dracCtxRefsLock.Lock()
	defer dracCtxRefsLock.Unlock()
	id := dracCtxRefsNextID
	dracCtxRefsNextID++

	dracCtxRefs[id] = dracCtx
	dracCtx.ctx = C.alloc_client_ctx(unsafe.Pointer(uintptr(id)), C.int(dracCtx.dracType))
}

func FreeDracClientCtx(dracCtx *DracCtx) {
	idAsPtr := C.get_client_data(dracCtx.ctx)
	id := int(uintptr(idAsPtr))

	dracCtxRefsLock.Lock()
	defer dracCtxRefsLock.Unlock()
	delete(dracCtxRefs, id)
	C.free_client_ctx(dracCtx.ctx)
}

func GetDracCtx(ctx *C.client_ctx) *DracCtx {
	idAsPtr := C.get_client_data(ctx)
	id := int(uintptr(idAsPtr))

	dracCtxRefsLock.Lock()
	defer dracCtxRefsLock.Unlock()
	return dracCtxRefs[id]
}

//export GlueReadDataCtrl
func GlueReadDataCtrl(ctx *C.client_ctx, size C.size_t) C.int {
	//unused

	// log.Printf("GlueReadDataCtrl %v\n", size)
	return 0
}

//export GlueWriteDataCtrl
func GlueWriteDataCtrl(ctx *C.client_ctx, data unsafe.Pointer, size C.size_t) C.int {
	bytes := (*[1 << 30]byte)(data)[:size:size]
	// b := C.GoBytes(data, C.int(size))
	// log.Printf("GlueWriteDataCtrl: %v\n", b)

	clientCtx := GetDracCtx(ctx)
	clientCtx.ctrlSocket.Write(bytes)
	return 0
}

//export GlueReadDataVideo
func GlueReadDataVideo(ctx *C.client_ctx, size C.size_t) C.int {
	//unused

	//log.Printf("GlueReadDataVideo (%v)\n", size)
	return 0
}

//export GlueWriteDataVideo
func GlueWriteDataVideo(ctx *C.client_ctx, data unsafe.Pointer, size C.size_t) C.int {
	bytes := (*[1 << 30]byte)(data)[:size:size]
	//bytes := C.GoBytes(data, C.int(size))
	//log.Printf("GlueWriteDataVideo: %v\n", bytes)

	clientCtx := GetDracCtx(ctx)
	clientCtx.videoSocket.Write(bytes)
	return 0
}

func sendKey(dracCtx *DracCtx, keycode int, keydown uint8) {
	C.send_key(dracCtx.ctx, C.uint32_t(keycode), C.uint8_t(keydown))
}

func sendMouse(dracCtx *DracCtx, x int, y int, buttonMask uint8, buttonEvent uint8) {
	C.send_mouse(dracCtx.ctx, C.int(x), C.int(y), C.uint8_t(buttonMask), C.uint8_t(buttonEvent))
}

func getFrame(dracCtx *DracCtx) (unsafe.Pointer, int, int) {
	frame := C.get_fb(dracCtx.ctx)
	height := int(C.get_height(dracCtx.ctx))
	width := int(C.get_width(dracCtx.ctx))

	return frame, width, height
}

func cFree(ptr unsafe.Pointer) {
	C.free(ptr)
}

func encodePngPtr(frame unsafe.Pointer, width int, height int) (unsafe.Pointer, int) {
	var pngData unsafe.Pointer
	pngDataSize := C.encode_png(frame, C.ushort(width), C.ushort(height), &pngData)
	if pngDataSize < 0 {
		log.Printf("png encode error!")
		return nil, -1
	}

	return pngData, int(pngDataSize)
}
