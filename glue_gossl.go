// +build !openssl

package main

import (
	"crypto/tls"
	"log"
	"net"
)

/*
#cgo CFLAGS: -Wno-deprecated-declarations -std=c99 -D_POSIX_C_SOURCE=200809L
#include "goglue.h"
*/
import "C"

//export GlueStartSslCtrl
func GlueStartSslCtrl(ctx *C.client_ctx) {
	clientCtx := (*DracCtx)(C.get_client_data(ctx))
	tlsconfig := &tls.Config{
		InsecureSkipVerify: true,
	}

	tlsConn := tls.Client(clientCtx.ctrlSocket, tlsconfig)
	//log.Printf("starting tls\n")
	err := tlsConn.Handshake()
	if err != nil {
		log.Printf("err: %v\n", err)
		return
	}
	//log.Printf("started tls\n")
	ctrlSocket := net.Conn(tlsConn)
	clientCtx.ctrlSocket = ctrlSocket
}

//export GlueStartSslVideo
func GlueStartSslVideo(ctx *C.client_ctx) {
	clientCtx := (*DracCtx)(C.get_client_data(ctx))
	tlsconfig := &tls.Config{
		InsecureSkipVerify: true,
	}
	tlsConn := tls.Client(clientCtx.videoSocket, tlsconfig)
	//log.Printf("starting tls\n")
	err := tlsConn.Handshake()
	if err != nil {
		log.Printf("err: %v\n", err)
		return
	}
	//log.Printf("started tls\n")
	videoSocket := net.Conn(tlsConn)
	clientCtx.videoSocket = videoSocket
}
