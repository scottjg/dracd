package main

import (
	"container/list"
	"fmt"
	"log"
	"net"
	"sync"
	"time"
	"unsafe"
)

/*
#include "goglue.h"
#include "protocol.h"
*/
import "C"

var dracmapLock = &sync.Mutex{}
var dracmap = make(map[string]*DracCtx)

type RecentDracEntry struct {
	Id            int
	Host          string
	Connected     bool
	StartTime     time.Time
	FormattedTime string

	Username string
	Password string
}

var recentDracs = list.List{}

type DracState struct {
	State string
	Time  time.Time
}

var dracstates = make(map[string]DracState)

func videoUpdate(dracCtx *DracCtx, r C.int) {
	var frame = (*Frame)(nil)
	if r == C.NEW_FRAME_READY {
		//t0 := time.Now()
		frameData, width, height := getFrame(dracCtx)

		//encode frame as png data to send to vnc clients
		pngData, pngDataSize := encodePngPtr(frameData, width, height)
		frame = newFrame(pngData, pngDataSize, width, height)

		// maybe later if someone requests an animated gif
		saveFrameForAnimation(dracCtx, frameData, width, height, frame)
		//log.Printf("*** encoded frame in %v\n", time.Since(t0))
	} else if r == C.VIDEO_IS_DOWN {
		frame = newFrameFromBytes(waiting_for_video_signal_png, 640, 480)
	}

	if frame != nil {
		//log.Printf("*** 2 sending a frame\n")
		for v := dracCtx.vncClients.Front(); v != nil; v = v.Next() {
			vncClient := v.Value.(*VncClientCtx)
			frame.acquire()
			vncClient.videoChannel <- frame
		}
		//log.Printf("*** 2 sent a frame\n")
		frame.release()

		//if !ready {
		dracstates[dracCtx.host] = DracState{
			State: "ready",
			Time:  time.Now(),
		}

		//	ready = true
		//}
	}

}

func videoReader(dracCtx *DracCtx, videoFinishedEvent chan int) {
	buf := make([]byte, 32768)
	//ready := false

	for {
		n, err := dracCtx.videoSocket.Read(buf)
		if n > 0 {
			//log.Printf("video read (%v) %v\n", n, buf[:n])
			r := C.incoming_data_video(dracCtx.ctx, (*C.uint8_t)(&buf[0]), C.size_t(n))
			//log.Printf("r=%v\n", r)
			if r == C.NEW_FRAME_READY || r == C.VIDEO_IS_DOWN {
				videoUpdate(dracCtx, r)
			}
		}

		if err != nil {
			log.Printf("video read error: %v\n", err)
			break
		}
	}

	videoFinishedEvent <- 1
}

func saveFrameForAnimation(dracCtx *DracCtx, frameData unsafe.Pointer, width int, height int, pngFrame *Frame) {
	qf := quantizeFrame(frameData, width, height)

	dracmapLock.Lock()
	defer dracmapLock.Unlock()

	oldFrame := dracCtx.lastPngFrame
	pngFrame.acquire()
	dracCtx.lastPngFrame = pngFrame
	if oldFrame != nil {
		oldFrame.release()
	}

	dracCtx.animatedFrameList.PushBack(AnimatedFrame{
		qf: qf,
		ts: time.Now(),
	})

	for dracCtx.animatedFrameList.Len() > 1 {
		e := dracCtx.animatedFrameList.Front()
		frame := e.Value.(AnimatedFrame)
		if time.Since(frame.ts) > (time.Duration(5) * time.Second) {
			dracCtx.animatedFrameList.Remove(e)
			freeQuantizedFrame(frame.qf)
		} else {
			break
		}
	}

	if !dracCtx.sentFirstFrame {
		close(dracCtx.firstFrameEvent)
		dracCtx.sentFirstFrame = true
	}
}

func ctrlReader(dracCtx *DracCtx, dracFinishedEvent chan int, login string, passwd string) {
	videoFinishedEvent := make(chan int)
	buf := make([]byte, 32768)

	login, passwd, dracType, ctrlPort, videoPort, err := getDracType(dracCtx.host, login, passwd)
	if err != nil {
		dracstates[dracCtx.host] = DracState{
			State: "connection error",
			Time:  time.Now(),
		}
		log.Printf("Failed to detect drac type for %s: %v", dracCtx.host, err)
	} else {
		log.Printf("Detected %s for %s", dracTypeString(dracType), dracCtx.host)
		dracCtx.dracType = dracType

		host := fmt.Sprintf("%s:%d", dracCtx.host, ctrlPort)
		socket, err := net.DialTimeout("tcp", host, 5*time.Second)
		if err != nil {
			dracstates[dracCtx.host] = DracState{
				State: "connection error",
				Time:  time.Now(),
			}
		} else {
			dracCtx.ctrlSocket = socket
			dracstates[dracCtx.host] = DracState{
				State: "connected",
				Time:  time.Now(),
			}
			AllocDracClientCtx(dracCtx)
			username := C.CString(login)
			password := C.CString(passwd)
			C.connect_start_ctrl(dracCtx.ctx, username, password)
			C.free(unsafe.Pointer(username))
			C.free(unsafe.Pointer(password))

			for {
				n, err := dracCtx.ctrlSocket.Read(buf)
				if n > 0 {
					//log.Printf("ctrl read (%v) %v\n", n, buf[:n])
					r := C.incoming_data_ctrl(dracCtx.ctx, (*C.uint8_t)(&buf[0]), C.size_t(n))
					//log.Printf("r=%v\n", r)
					if r == C.NEED_TO_CONNECT_VIDEO {
						host := fmt.Sprintf("%s:%d", dracCtx.host, videoPort)
						videoSocket, err := net.Dial("tcp", host)
						if err != nil {
							dracstates[dracCtx.host] = DracState{
								State: "video connection error",
								Time:  time.Now(),
							}
							dracCtx.ctrlSocket.Close()
							break
						}

						dracstates[dracCtx.host] = DracState{
							State: "video connected",
							Time:  time.Now(),
						}
						dracCtx.videoSocket = videoSocket
						go videoReader(dracCtx, videoFinishedEvent)
						C.connect_start_video(dracCtx.ctx)
					} else if r <= C.ERR_LOGIN_FAILURE && r >= C.ERR_LOGIN_FAILURE_MAX {
						dracstates[dracCtx.host] = DracState{
							State: "auth error",
							Time:  time.Now(),
						}
					} else if r == C.NEW_FRAME_READY || r == C.VIDEO_IS_DOWN {
						videoUpdate(dracCtx, r)
					}
				}

				if err != nil {
					log.Printf("ctrl read error: %v\n", err)
					break
				}
			}
		}
	}

	if dracCtx.videoSocket != nil {
		dracCtx.videoSocket.Close()
		<-videoFinishedEvent //wait for video channel to terminate
	}

	dracFinishedEvent <- 1
}

func dracHandler(dracCtx *DracCtx, login string, passwd string) {
	dracFinishedEvent := make(chan int)
	go ctrlReader(dracCtx, dracFinishedEvent, login, passwd)

	dracFinished := false

loop:
	for {
		select {
		case <-dracFinishedEvent:
			dracmapLock.Lock()
			dracFinished = true
			delete(dracmap, dracCtx.host)
			if dracCtx.otherClientCount <= 0 && dracCtx.vncClients.Len() <= 0 {
				dracmapLock.Unlock()
				break loop
			}

			for v := dracCtx.vncClients.Front(); v != nil; v = v.Next() {
				teardownVncClient(v.Value.(*VncClientCtx))
			}
			dracmapLock.Unlock()
		case <-dracCtx.clientEvent:
			if dracFinished {
				dracmapLock.Lock()
				b := dracCtx.otherClientCount <= 0 && dracCtx.vncClients.Len() <= 0
				dracmapLock.Unlock()
				if b {
					break loop
				}
			}
		case <-time.After(time.Second * 30):
			dracmapLock.Lock()
			b := dracCtx.otherClientCount <= 0 && dracCtx.vncClients.Len() <= 0
			dracmapLock.Unlock()
			if b && dracCtx.ctrlSocket != nil {
				dracCtx.ctrlSocket.Close()
			}
		}
	}

	if !dracCtx.sentFirstFrame {
		close(dracCtx.firstFrameEvent)
		dracCtx.sentFirstFrame = true
	}

	f := dracCtx.animatedFrameList.Front()
	for f != nil {
		g := f.Next()
		dracCtx.animatedFrameList.Remove(f)
		freeQuantizedFrame(f.Value.(AnimatedFrame).qf)
		f = g
	}

	pngFrame := dracCtx.lastPngFrame
	dracCtx.lastPngFrame = nil
	if pngFrame != nil {
		pngFrame.release()
	}

	if dracCtx.ctx != nil {
		FreeDracClientCtx(dracCtx)
	}
}

func getRecentDrac(host string) *RecentDracEntry {
	dracmapLock.Lock()
	defer dracmapLock.Unlock()
	for dracEntry := recentDracs.Front(); dracEntry != nil; dracEntry = dracEntry.Next() {
		if dracEntry.Value.(RecentDracEntry).Host == host {
			de := dracEntry.Value.(RecentDracEntry)
			return &de
		}
	}

	return nil
}

func setupDracClient(vncCtx *VncClientCtx, host string, login string, passwd string) (dracCtx *DracCtx) {
	dracmapLock.Lock()
	dracCtx = dracmap[host]
	if dracCtx == nil {
		for dracEntry := recentDracs.Front(); dracEntry != nil; dracEntry = dracEntry.Next() {
			de := dracEntry.Value.(RecentDracEntry)
			if de.Host == host {
				if login == "" {
					login = de.Username
				}
				if passwd == "" {
					passwd = de.Password
				}
				recentDracs.Remove(dracEntry)
			}
		}
		recentDracs.PushFront(
			RecentDracEntry{
				Host:      host,
				StartTime: time.Now(),
				Username:  login,
				Password:  passwd,
			},
		)
		if recentDracs.Len() > 10 {
			recentDracs.Remove(recentDracs.Back())
		}

		dracCtx = &DracCtx{}
		dracmap[host] = dracCtx

		dracCtx.host = host
		dracCtx.vncClients.Init()
		dracCtx.animatedFrameList.Init()
		dracCtx.firstFrameEvent = make(chan int)
		dracCtx.sentFirstFrame = false
		dracCtx.startTime = time.Now()

		dracCtx.otherClientCount = 0
		dracCtx.clientEvent = make(chan int)

		if vncCtx != nil {
			vncCtx.videoChannel <- newFrameFromBytes(connecting_png, 640, 480)
		}

		dracstates[dracCtx.host] = DracState{
			State: "connecting",
			Time:  time.Now(),
		}
		go dracHandler(dracCtx, login, passwd)
	} else {
		if vncCtx != nil && !dracCtx.sentFirstFrame {
			vncCtx.videoChannel <- newFrameFromBytes(connecting_png, 640, 480)
		}
	}

	if vncCtx != nil {
		dracCtx.vncClients.PushBack(vncCtx)
		vncCtx.dracCtx = dracCtx
	} else {
		dracCtx.otherClientCount++
	}

	dracmapLock.Unlock()
	dracCtx.clientEvent <- 1
	return dracCtx
}

func teardownDracRequest(dracCtx *DracCtx) {
	dracmapLock.Lock()
	dracCtx.otherClientCount--
	dracmapLock.Unlock()

	dracCtx.clientEvent <- 1
}

func teardownDracClient(vncCtx *VncClientCtx) {
	dracmapLock.Lock()
	dracCtx := vncCtx.dracCtx
	vncCtx.dracCtx = nil

	log.Printf("tearing down...\n")
	for v := dracCtx.vncClients.Front(); v != nil; v = v.Next() {
		log.Printf("%+v\n", v.Value)
		if v.Value == vncCtx {
			dracCtx.vncClients.Remove(v)
			close(vncCtx.videoChannel)
			break
		}
	}
	dracmapLock.Unlock()

	dracCtx.clientEvent <- 1
}
