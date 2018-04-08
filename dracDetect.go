package main

import (
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/publicsuffix"
)

type DracType int

const (
	DRAC4 = 4
	DRAC5 = 5
	DRAC6 = 6
	DRAC7 = 7
	DRAC8 = 8

	UNKNOWN = -1
)

func getDracType(host string, user string, password string) (sessionUser string, sessionPassword string, dracType DracType, ctrlPort int, videoPort int, err error) {
	timeout := time.Duration(30 * time.Second)
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
			//buggy drac4 won't work if we negotiate higher max version
			MaxVersion: tls.VersionTLS10,
		},
	}

	options := cookiejar.Options{
		PublicSuffixList: publicsuffix.List,
	}

	jar, err := cookiejar.New(&options)
	if err != nil {
		log.Fatal(err)
	}

	client := http.Client{
		Timeout:   timeout,
		Transport: tr,
		Jar:       jar,
	}

	req, err := http.NewRequest("GET", "https://" + host + "/", nil)
	if err != nil {
		return "", "", -1, -1, -1, err
	}
	// some dracs have this bug where it will hold the connection open
	// and send a bogus content length, which causes this client to hang
	req.Header.Add("Connection", "Close")

	resp, err := client.Do(req)
	if err != nil {
		return "", "", -1, -1, -1, err
	}
	defer resp.Body.Close()
	bytes, err := ioutil.ReadAll(resp.Body)
	body := string(bytes[:])

	if strings.Contains(body, "iDrac8") {
		dracType = DRAC8
	} else if strings.Contains(body, "top.document.location.href = \"/index.html?console") {
		dracType = DRAC7
	} else if strings.Contains(body, "top.document.location.href = \"/sclogin") {
		dracType = DRAC6
	} else if strings.Contains(body, "top.document.location.replace(\"/cgi-bin/webcgi/index\");") {
		dracType = DRAC5
	} else if strings.Contains(body, "var s_oemProductName = \"DRAC 4\";") {
		dracType = DRAC4
	} else {
		dracType = UNKNOWN
	}

	switch {
	case dracType >= DRAC6:
		return user, password, dracType, 5900, 5900, nil

	case dracType == DRAC5:
		resp, err := client.PostForm("https://"+host+"/cgi-bin/webcgi/login",
			url.Values{"user": {user}, "password": {password}})
		if err != nil {
			return "", "", -1, -1, -1, err
		}
		defer client.PostForm("https://"+host+"/cgi-bin/webcgi/logout", url.Values{})
		defer resp.Body.Close()
		bytes, err := ioutil.ReadAll(resp.Body)
		body := string(bytes[:])
		log.Printf(body)

		resp, err = client.Get("https://" + host + "/cgi-bin/webcgi/vkvm?state=1")
		if err != nil {
			return "", "", -1, -1, -1, err
		}
		defer resp.Body.Close()
		bytes, err = ioutil.ReadAll(resp.Body)
		body = string(bytes[:])
		log.Printf(body)

		sessionId := getValue(body, "<property name=\"vKvmSessionId\" type=\"string\"><value>")
		//encryptionEnabled := getValue(body, "<property name=\"EncryptionEnabled\" type=\"boolean\"><value>")
		ctrlPortStr := getValue(body, "<property name=\"KMPortNumber\" type=\"text\"><value>")
		videoPortStr := getValue(body, "<property name=\"VideoPortNumber\" type=\"text\"><value>")
		ctrlPort, _ := strconv.Atoi(ctrlPortStr)
		videoPort, _ := strconv.Atoi(videoPortStr)

		log.Printf("sessionId=%s ctrlPort=%d videoPort=%d", sessionId, ctrlPort, videoPort)
		return sessionId, "", dracType, ctrlPort, videoPort, nil

	case dracType == DRAC4:
		resp, err := client.PostForm("https://"+host+"/cgi/login",
			url.Values{"user": {user}, "hash": {password}})
		if err != nil {
			return "", "", -1, -1, -1, err
		}
		defer client.PostForm("https://"+host+"/cgi/logout", url.Values{})
		defer resp.Body.Close()
		bytes, err := ioutil.ReadAll(resp.Body)
		body := string(bytes[:])
		log.Printf(body)

		resp, err = client.Get("https://" + host + "/cgi/vkvm")
		if err != nil {
			return "", "", -1, -1, -1, err
		}
		defer resp.Body.Close()
		bytes, err = ioutil.ReadAll(resp.Body)
		body = string(bytes[:])
		log.Printf(body)

		sessionNum := getValue(body, "var nSessionNum = ")
		sessionId := getValue(body, "var sSessionId = \"")
		ctrlPortStr := getValue(body, "var nPort = ")
		ctrlPort, _ := strconv.Atoi(ctrlPortStr)

		log.Printf("sessionNum=%s sessionId=%s ctrlPort=%d", sessionNum, sessionId, ctrlPort)
		return sessionNum, sessionId, dracType, ctrlPort, -1, nil
	default:
		panic("unimplemented")

	}
}

func getValue(str string, prefix string) string {
	i := strings.Index(str, prefix)
	if i < 0 {
		return ""
	}

	i += len(prefix)
	j := i + 1
	for ; j < len(str); j++ {
		if str[j] == '<' || str[j] == ';' || str[j] == '"' {
			break
		}
	}

	return str[i:j]
}

func dracTypeString(dracType DracType) string {
	if dracType == UNKNOWN {
		return "Unknown"
	} else {
		return fmt.Sprintf("DRAC%d", dracType)
	}
}
