# dracd

This was an experiment I wrote awhile ago as a daemon you can run that provides some additional funtionality for the Dell Remote Access Console on PowerEdge model servers

1. It provides a DRAC protocol to VNC bridge using a noVNC javascript client. This lets you connect to DRACs without using the java client. Nowadays this is not as useful since modern Dell servers provide a javascript client too, but it's nice for older servers. It also had good support for sharing the console. If two people connected to the same server, they could both see the screen and control it. The interface to use the js drac console is exposed on http://localhost:8686/
2. It provides an API to grab screenshots and animated gifs from a server via the DRAC. For example, if your drac ip is 1.2.3.4, you can fetch a screenshot with http://localhost:8686/1.2.3.4.png. it assumes the default drac password (root/calvin).

### Building

The code relied on a C library decoder for DRAC that I wrote that is shared with [System Scope](https://getsystemscope.com/), a native mac DRAC client. It uses libpng for video encoding, giflib for gif encoding, and openssl to connect to older DRACs that used deprecated ciphers. For ease of development, you can build without openssl or giflib. To enable support for openssl and gifs, you must enable the build tags.

First, setup a gopath with the package
```
$ mkdir -p go/src/github.com/scottjg
$ cd go
$ export GOPATH=`pwd`
$ cd src/github.com/scottjg
$ git clone https://github.com/scottjg/dracd
$ cd dracd
```

Then, you can build the basic daemon:
```
$ go get
```

Then you can just run the daemon
```
$ ./dracd
```

Alternatively, you can build with the additional features if you have the dependencies installed on your sytsem
```
$ go get -tags 'openssl gif'
```


![](https://cl.ly/373d1V1y2b2y/Screen%20Shot%202018-07-15%20at%201.35.03%20PM.png)
![](https://cl.ly/0p2u1v2V2P3p/Screen%20Shot%202018-07-15%20at%201.39.56%20PM.png)
![](https://cl.ly/0i2A0J2a1l1d/Screen%20Shot%202018-07-15%20at%201.35.53%20PM.png)
