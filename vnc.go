package main

import (
	"bufio"
	"encoding/binary"
	"log"
	"net"
	"strconv"
	"sync"
)

type PixelFmt struct {
	Bpp        uint8
	Depth      uint8
	BigEndian  uint8
	TrueColor  uint8
	RedMax     uint16
	GreenMax   uint16
	BlueMax    uint16
	RedShift   uint8
	GreenShift uint8
	BlueShift  uint8
}

type VncClientCtx struct {
	lock     sync.Mutex
	waiting  bool
	framegen int
	bw       *bufio.Writer
	pf       PixelFmt
	width    int
	height   int
	dracCtx  *DracCtx
	conn     net.Conn

	videoChannel     chan *Frame
	sendVideoChannel chan int
	stopVideoChannel chan int
}

func handleVncConnection(client net.Conn, dracHost string) {
	defer client.Close()
	br := bufio.NewReader(client)
	bw := bufio.NewWriter(client)
	log.Printf("Got connection from %v\n", client.RemoteAddr())

	bw.WriteString("RFB 003.008\n")
	bw.Flush()
	requestedVersion, err := br.ReadSlice('\n')
	if err != nil || len(requestedVersion) < 12 {
		log.Printf("Failed to read version string from client (%v)\n", err)
		return
	}

	str := string(requestedVersion[:11])
	ver, err := strconv.Atoi(str[8:11])
	if err != nil || str[:8] != "RFB 003." {
		log.Printf("Got bad protocol version: %v\n", str)
		return
	}

	log.Printf("Got version: %d (%v)\n", ver, str)

	if ver >= 7 {
		// no auth
		bw.WriteString("\x01\x01")
		bw.Flush()

		b, err := br.ReadByte()
		if err != nil {
			log.Printf("error reading security type\n")
			return
		}
		if b != 1 {
			log.Printf("error: client requested unknown auth type %d\n", int(b))
		}
	} else {
		binary.Write(bw, binary.BigEndian, uint32(1))
		bw.Flush()
	}

	if ver >= 8 {
		// send security result (ok)
		binary.Write(bw, binary.BigEndian, uint32(0))
		bw.Flush()
	}

	b, err := br.ReadByte()
	if err != nil {
		log.Printf("error reading shared flag\n")
		return
	}

	log.Printf("shared flag: %d\n", b)

	clientCtx := VncClientCtx{
		waiting:  false,
		framegen: 0,
		bw:       bw,
		conn:     client,
		pf: PixelFmt{
			Bpp:        32,
			Depth:      24,
			BigEndian:  0,
			TrueColor:  1,
			RedMax:     255,
			GreenMax:   255,
			BlueMax:    255,
			RedShift:   0,
			GreenShift: 8,
			BlueShift:  16,
		},
		width:            640,
		height:           480,
		videoChannel:     make(chan *Frame),
		sendVideoChannel: make(chan int),
		stopVideoChannel: make(chan int),
	}

	binary.Write(bw, binary.BigEndian, uint16(clientCtx.width))
	binary.Write(bw, binary.BigEndian, uint16(clientCtx.height))
	binary.Write(bw, binary.BigEndian, clientCtx.pf.Bpp)
	binary.Write(bw, binary.BigEndian, clientCtx.pf.Depth)
	binary.Write(bw, binary.BigEndian, clientCtx.pf.BigEndian)
	binary.Write(bw, binary.BigEndian, clientCtx.pf.TrueColor)
	binary.Write(bw, binary.BigEndian, clientCtx.pf.RedMax)
	binary.Write(bw, binary.BigEndian, clientCtx.pf.GreenMax)
	binary.Write(bw, binary.BigEndian, clientCtx.pf.BlueMax)
	binary.Write(bw, binary.BigEndian, clientCtx.pf.RedShift)
	binary.Write(bw, binary.BigEndian, clientCtx.pf.GreenShift)
	binary.Write(bw, binary.BigEndian, clientCtx.pf.BlueShift)

	//padding
	binary.Write(bw, binary.BigEndian, uint8(0))
	binary.Write(bw, binary.BigEndian, uint8(0))
	binary.Write(bw, binary.BigEndian, uint8(0))

	//server banner
	binary.Write(bw, binary.BigEndian, uint32(len(dracHost)))
	bw.WriteString(dracHost)
	bw.Flush()

	videoWriterStopSignal := make(chan int)
	go bufferedVideoWriter(&clientCtx, videoWriterStopSignal)
	setupDracClient(&clientCtx, dracHost, "", "")
	defer teardownDracClient(&clientCtx)

	var oldButtonMask uint8

	for {
		cmd, err := br.ReadByte()
		if err != nil {
			log.Printf("error reading client cmd\n")
			break
		}

		switch cmd {
		case 0: //set pixel format
			br.ReadByte()
			br.ReadByte()
			br.ReadByte()
			binary.Read(br, binary.BigEndian, &clientCtx.pf.Bpp)
			binary.Read(br, binary.BigEndian, &clientCtx.pf.Depth)
			binary.Read(br, binary.BigEndian, &clientCtx.pf.BigEndian)
			binary.Read(br, binary.BigEndian, &clientCtx.pf.TrueColor)
			binary.Read(br, binary.BigEndian, &clientCtx.pf.RedMax)
			binary.Read(br, binary.BigEndian, &clientCtx.pf.GreenMax)
			binary.Read(br, binary.BigEndian, &clientCtx.pf.BlueMax)
			binary.Read(br, binary.BigEndian, &clientCtx.pf.RedShift)
			binary.Read(br, binary.BigEndian, &clientCtx.pf.GreenShift)
			binary.Read(br, binary.BigEndian, &clientCtx.pf.BlueShift)
			br.ReadByte()
			br.ReadByte()
			br.ReadByte()
			log.Printf("pixel format: %+v\n", clientCtx.pf)

		case 2: //set encodings
			var count uint16
			br.ReadByte()
			binary.Read(br, binary.BigEndian, &count)
			log.Printf("got set encoding message: %v\n", count)
			for ; count > 0; count-- {
				var encoding int32
				binary.Read(br, binary.BigEndian, &encoding)
				log.Printf("got encoding %d\n", encoding)
			}

		case 3: //framebuffer update request
			//log.Printf("fb update req\n")
			padding := make([]byte, 9)
			binary.Read(br, binary.BigEndian, padding)

			//clientCtx.waiting = true
			//sendVideoFrame(&clientCtx)
			//log.Printf("*** got fb update req!\n")
			clientCtx.sendVideoChannel <- 1
			//log.Printf("*** sent fb update signal!\n")
		case 4: //key event
			var downFlag uint8
			var padding uint16
			var key uint32
			binary.Read(br, binary.BigEndian, &downFlag)
			binary.Read(br, binary.BigEndian, &padding)
			binary.Read(br, binary.BigEndian, &key)
			//log.Printf("got key event: %x (%d)\n", downFlag)

			var usbkeycode int
			if key >= 0 && key <= 255 {
				usbkeycode = x11ToUsb1[key]
			} else if key >= 0xFF00 && key <= 0xFFFF {
				usbkeycode = x11ToUsb2[key-0xFF00]
			} else {
				usbkeycode = -1
			}
			if usbkeycode >= 0 {
				//XXX lock per drac? writer channel?
				sendKey(clientCtx.dracCtx, usbkeycode, downFlag)
			}

		case 5: //mouse event
			var buttonMask uint8
			var xpos, ypos uint16
			binary.Read(br, binary.BigEndian, &buttonMask)
			binary.Read(br, binary.BigEndian, &xpos)
			binary.Read(br, binary.BigEndian, &ypos)
			//log.Printf("*** mouse event\n")
			if buttonMask != oldButtonMask {
				//XXX lock per drac? writer channel?
				sendMouse(clientCtx.dracCtx, int(xpos), int(ypos), buttonMask, 1)
				oldButtonMask = buttonMask
			} else {
				sendMouse(clientCtx.dracCtx, int(xpos), int(ypos), buttonMask, 0)
			}
		default:
			log.Printf("got msg: %v\n", cmd)
			break
		}

	}
	videoWriterStopSignal <- 1
}

func bufferedVideoWriter(vncCtx *VncClientCtx, stopSignal chan int) {
	//buffers latest incoming drac video frame while we wait for the vnc client to drain a video frame
	videoStopSignal := make(chan int)
	frameChannel := make(chan *Frame)

	go videoWriter(vncCtx, frameChannel, videoStopSignal)
	var frame *Frame

loop:
	for {
		//log.Printf("*** 1 (%p) waiting to buffer a frame\n", vncCtx)
		select {
		case frame = <-vncCtx.videoChannel:
			//log.Printf("*** 1 buffered a frame\n")
		case _ = <-stopSignal:
			break loop
		}
		//log.Printf("*** 1 (%p) waiting to send a frame\n", vncCtx)
		for {
			select {
			case _ = <-stopSignal:
				break loop
			case newFrame := <-vncCtx.videoChannel:
				//log.Printf("*** 1 (%p) buffered a frame (overrun)\n", vncCtx)
				frame.release()
				frame = newFrame
			case frameChannel <- frame:
				//log.Printf("*** 1 (%p) sent a frame to vnc writer\n", vncCtx)
				//xfer the ref to the vnc frame writer
				frame = nil
				continue loop
			}
		}
	}

	if frame != nil {
		frame.release()
	}

	videoStopSignal <- 1
	//log.Printf("*** 1 frameBufferWriter ended\n")
}

func videoWriter(clientCtx *VncClientCtx, frameChannel chan *Frame, stopSignal chan int) {
	pendingVideoUpdates := 0
loop:
	for {
		if pendingVideoUpdates < 1 {
			//log.Printf("*** waiting for video signal...\n")
			select {
			case <-clientCtx.sendVideoChannel:
				pendingVideoUpdates++
				break
			case <-stopSignal:
				//log.Printf("*** got stop signal!")
				break loop
			}
			//log.Printf("*** got video signal...\n")
		}
		select {
		case <-clientCtx.sendVideoChannel:
			//we already got the signal. drain it otherwise
			//it blocks the sending goroutine
			//log.Printf("got spurious video update signal\n")

			// we sometimes send two fb updates: one for changing resolution,
			// and one for sending the actual video data. so we might also get
			// a legit second fb update req here, so we don't want to completely
			// forget that it's safe to send another frame now
			pendingVideoUpdates++
		case frame := <-frameChannel:
			//log.Printf("**** sending new frame\n")
			sendVncFrame(clientCtx, frame.data, frame.width, frame.height)
			frame.release()
			pendingVideoUpdates--
			if pendingVideoUpdates > 0 {
				//log.Printf("**** pendingVideoUpdates=%d", pendingVideoUpdates)
			} else {
				//log.Printf("**** no more pending video updates")
				break
			}
		case <-stopSignal:
			//log.Printf("*** got stop signal!")
			break loop
		}
		//log.Printf("*** got frame\n")
	}
	//log.Printf("*** video writer is done!")
}

func sendVncFrame(clientCtx *VncClientCtx, frame []byte, width int, height int) {
	if clientCtx.height != height || clientCtx.width != width {
		binary.Write(clientCtx.bw, binary.BigEndian, uint8(0))       //msg type
		binary.Write(clientCtx.bw, binary.BigEndian, uint8(0))       //padding
		binary.Write(clientCtx.bw, binary.BigEndian, uint16(1))      //num rects
		binary.Write(clientCtx.bw, binary.BigEndian, uint16(0))      //x
		binary.Write(clientCtx.bw, binary.BigEndian, uint16(0))      //y
		binary.Write(clientCtx.bw, binary.BigEndian, uint16(width))  //width
		binary.Write(clientCtx.bw, binary.BigEndian, uint16(height)) //height
		binary.Write(clientCtx.bw, binary.BigEndian, int32(-223))    //encoding

		clientCtx.height = height
		clientCtx.width = width
	}

	binary.Write(clientCtx.bw, binary.BigEndian, uint8(0))       //msg type
	binary.Write(clientCtx.bw, binary.BigEndian, uint8(0))       //padding
	binary.Write(clientCtx.bw, binary.BigEndian, uint16(1))      //num rects
	binary.Write(clientCtx.bw, binary.BigEndian, uint16(0))      //x
	binary.Write(clientCtx.bw, binary.BigEndian, uint16(0))      //y
	binary.Write(clientCtx.bw, binary.BigEndian, uint16(width))  //width
	binary.Write(clientCtx.bw, binary.BigEndian, uint16(height)) //height
	binary.Write(clientCtx.bw, binary.BigEndian, int32(7))       //encoding

	binary.Write(clientCtx.bw, binary.BigEndian, uint8(0xa0))

	size := len(frame)
	var csize []byte
	if size <= 127 {
		csize = make([]byte, 1)
		csize[0] = byte(size)
	} else if size <= 16383 {
		csize = make([]byte, 2)
		csize[0] = byte((size & 0x7f) | 0x80)
		csize[1] = byte((size >> 7) & 0x7f)
	} else if size <= 4194303 {
		csize = make([]byte, 3)
		csize[0] = byte((size & 0x7f) | 0x80)
		csize[1] = byte(((size >> 7) & 0x7f) | 0x80)
		csize[2] = byte((size >> 14) & 0x7f)
	}
	binary.Write(clientCtx.bw, binary.BigEndian, csize)
	binary.Write(clientCtx.bw, binary.BigEndian, frame)
	clientCtx.bw.Flush()
}

func teardownVncClient(vncCtx *VncClientCtx) {
	vncCtx.conn.Close()
}
