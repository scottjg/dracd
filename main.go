package main

import (
	"log"
	//"net"
	//"runtime"
	//"github.com/shurcooL/trayhost"
)

import _ "net/http/pprof"

func main() {
	log.Printf("hello.\n")

	//go func() {
	//	server, err := net.Listen("tcp", ":5900")
	//	if err != nil {
	//		panic(err)
	//	}

	//	for {
	//		conn, err := server.Accept()
	//		if err != nil {
	//			log.Printf("accept error: %v\n", err)
	//		}
	//		go handleVncConnection(conn, "69.41.163.35")
	//	}
	//}()
	ServeHttp()

	//runtime.LockOSThread()
	//trayhost.SetUrl("http://localhost:8686")
	//trayhost.EnterLoop()
}
