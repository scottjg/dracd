package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"
	"github.com/gorilla/websocket"
)

var secret = securecookie.GenerateRandomKey(32)
var store = sessions.NewFilesystemStore(".", secret)

func SetupSession(writer http.ResponseWriter, req *http.Request, user string) {
	session, _ := store.Get(req, "session")
	session.Options.Path = "/"
	if session.Values["csrfToken"] == nil {
		session.Values["csrfToken"] = hex.EncodeToString(securecookie.GenerateRandomKey(32))
	}
	session.Values["user"] = user
	session.Save(req, writer)
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  32768,
	WriteBufferSize: 32768,
	CheckOrigin:     func(r *http.Request) bool { return true },
	Subprotocols:    []string{"binary"},
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", 405)
		return
	}
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer ws.Close()

	ip := mux.Vars(r)["ip"]

	wsc := CreateWsConn(ws)
	handleVncConnection(wsc, ip)
}

func gifHostHandler(w http.ResponseWriter, r *http.Request) {
	ip := mux.Vars(r)["ip"]

	dracCtx := setupDracClient(nil, ip, "", "")
	defer teardownDracRequest(dracCtx)
	<-dracCtx.firstFrameEvent

	dracmapLock.Lock()
	defer dracmapLock.Unlock()
	if dracCtx.animatedFrameList.Len() == 0 {
		http.Error(w, "Internal Server Error", 500)
		return
	}

	t0 := time.Now()
	gifData, size := encodeGif(dracCtx.animatedFrameList)
	d := (*[1 << 30]byte)(gifData)[:size:size]
	log.Printf("writing gif of size %d took %v\n", size, time.Since(t0))
	w.Write(d)
}

func pngHostHandler(w http.ResponseWriter, r *http.Request) {
	ip := mux.Vars(r)["ip"]

	dracCtx := setupDracClient(nil, ip, "", "")
	defer teardownDracRequest(dracCtx)
	<-dracCtx.firstFrameEvent

	dracmapLock.Lock()
	defer dracmapLock.Unlock()
	if dracCtx.animatedFrameList.Len() == 0 {
		http.Error(w, "Internal Server Error", 500)
		return
	}

	w.Write(dracCtx.lastPngFrame.data)
}

func statusHandler(w http.ResponseWriter, r *http.Request) {
	ip := mux.Vars(r)["ip"]
	dracmapLock.Lock()
	defer dracmapLock.Unlock()
	drac, ok := dracstates[ip]
	state := "disconnected"
	//log.Printf("%v %+v\n", ip, drac)
	//for k := range dracmap {
	//	log.Printf("%v\n", k)
	//}
	if ok {
		state = drac.State
	}

	fmt.Fprintf(w, "{\"status\":\"%s\"}", state)
}

func connectHandler(w http.ResponseWriter, r *http.Request) {
	tmplData := struct {
		RecentDracs   []RecentDracEntry
		ConnectToHost string
	}{}
	tmplData.ConnectToHost = ""

	if r.Method == "POST" {
		var host, login, password string

		host = r.FormValue("host")
		if host == "" { // assume json payload
			var data map[string]interface{}
			body, _ := ioutil.ReadAll(r.Body)
			_ = json.Unmarshal([]byte(body), &data)
			host = data["host"].(string)
			login, _ = data["login"].(string)
			password, _ = data["password"].(string)
		} else { // otherwise assume form encoded payload
			login = r.FormValue("login")
			password = r.FormValue("password")
		}

		if login == "" || password == "" {
			dracEntry := getRecentDrac(host)
			if dracEntry == nil {
				fmt.Fprintf(w, "{\"status\":\"need credentials\"}")
				return
			} else {
				login = dracEntry.Username
				password = dracEntry.Password
			}
		}

		ctx := setupDracClient(nil, host, login, password)
		teardownDracRequest(ctx)

		if r.Header.Get("X-Requested-With") == "XMLHttpRequest" {
			fmt.Fprintf(w, "{\"status\":\"connecting\"}")
			return
		}

		tmplData.ConnectToHost = host
	}

	t, _ := template.New("connect").Parse(connect_html)
	//t, _ := template.ParseFiles("connect.html")
	dracmapLock.Lock()
	defer dracmapLock.Unlock()
	i := 0
	for entry := recentDracs.Front(); entry != nil; entry = entry.Next() {
		drac := entry.Value.(RecentDracEntry)
		if dracmap[drac.Host] != nil {
			drac.Connected = true
		} else {
			drac.Connected = false
		}
		drac.FormattedTime = drac.StartTime.Format("1/2 15:04 (MST)")
		drac.Id = i
		i++
		tmplData.RecentDracs = append(tmplData.RecentDracs, drac)
	}
	fmt.Printf("%+v\n", tmplData)
	t.Execute(w, tmplData)
}

func killSessionHandler(w http.ResponseWriter, r *http.Request) {
	host := r.FormValue("host")

	dracmapLock.Lock()
	defer dracmapLock.Unlock()
	if host != "" {
		drac := dracmap[host]
		if drac != nil {
			drac.ctrlSocket.Close()
		}
	}
	fmt.Fprintf(w, "ok!\n")
}

func ServeHttp() {
	r := mux.NewRouter()
	r.NotFoundHandler = http.FileServer(http.Dir("."))
	r.HandleFunc("/", connectHandler)
	r.HandleFunc("/killsession", killSessionHandler)
	r.HandleFunc("/status/{ip:.*}", statusHandler)
	r.HandleFunc("/websockify/{ip:.*}", wsHandler)
	if GIF_SUPPORT_ENABLED {
		r.HandleFunc("/{ip:.*}.gif", gifHostHandler)
	}
	r.HandleFunc("/{ip:.*}.png", pngHostHandler)
	routeAssets(r)
	http.Handle("/", r)
	http.ListenAndServe(":8686", nil)
}

func debugSession(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session")
	log.Printf("%v\n", session)
}
