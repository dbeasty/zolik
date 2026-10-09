// Command phonehost runs the server the phone app embeds, on a desktop, for
// testing what the phone serves without a phone: the same zolikcore package,
// the same routes, a loopback listener for "the phone's own app" and the room
// listener a browser on the network reaches.
//
//	go run ./cmd/phonehost -data /tmp/phone -web ../client-react-native/dist
//
// With -credential and -user it starts as an enrolled phone with somebody
// signed in, replicating with -cloud, exactly as zolikcore.StartNode does on a
// device. With -enroll-token (an account's access token on -cloud) and -user
// it enrols itself first, the way the app does at sign-in. With -relay it
// also opens the table to the internet through -cloud, as the app's
// "Let people join over the internet" does. It prints one line of JSON with
// its addresses (and relay code) once it is up, and runs until interrupted.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"zolik/server/mobile/zolikcore"
)

func main() {
	data := flag.String("data", "", "data directory (required)")
	web := flag.String("web", "", "folder holding the web client export to serve")
	cloud := flag.String("cloud", "", "cloud base URL, e.g. http://127.0.0.1:8090")
	credential := flag.String("credential", "", "node credential from /nodes/enroll")
	user := flag.String("user", "", "account id signed in on this phone")
	room := flag.Bool("room", true, "open the room listener")
	enrollToken := flag.String("enroll-token", "", "an account's access token on -cloud, to enrol this host with")
	relayName := flag.String("relay", "", "open the table to the internet through -cloud's relay, under this name")
	flag.Parse()
	if *data == "" {
		log.Fatal("phonehost: -data is required")
	}
	if *web != "" {
		zolikcore.SetWebRoot(*web)
	}
	if *enrollToken != "" {
		cred, err := enroll(*data, *cloud, *enrollToken)
		if err != nil {
			log.Fatalf("phonehost: enrolling: %v", err)
		}
		*credential = cred
	}
	h, err := zolikcore.StartNode(*data, *credential, *user, *cloud)
	if err != nil {
		log.Fatalf("phonehost: %v", err)
	}
	if *cloud != "" {
		if err := h.RefreshCloudKeys(); err != nil {
			log.Printf("phonehost: fetching the cloud's keys: %v", err)
		}
	}
	out := map[string]any{
		"baseUrl":       h.BaseURL(),
		"instanceId":    h.InstanceID(),
		"nodeId":        h.NodeID(),
		"nodePublicKey": h.NodePublicKey(),
	}
	if *room {
		port, err := h.OpenLAN()
		if err != nil {
			log.Fatalf("phonehost: %v", err)
		}
		out["lanPort"] = port
		out["roomUrl"] = "http://127.0.0.1:" + strconv.Itoa(port)
	}
	if *relayName != "" {
		code, err := h.OpenRelay(*relayName)
		if err != nil {
			log.Fatalf("phonehost: opening the relay: %v", err)
		}
		out["relayCode"] = code
		out["relayUrl"] = h.RelayURL()
	}
	_ = json.NewEncoder(os.Stdout).Encode(out)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	h.Stop()
}

// enroll starts the host once to learn its node key, enrols that key with the
// cloud as the account the token belongs to, and returns the credential.
func enroll(data, cloud, token string) (string, error) {
	h, err := zolikcore.Start(data)
	if err != nil {
		return "", err
	}
	pub, instance := h.NodePublicKey(), h.InstanceID()
	h.Stop()
	body, _ := json.Marshal(map[string]string{"pubkey": pub, "kind": "phone", "instanceId": instance})
	req, _ := http.NewRequest(http.MethodPost, cloud+"/nodes/enroll", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	var out struct {
		Credential string `json:"credential"`
	}
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("/nodes/enroll: %s", res.Status)
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil || out.Credential == "" {
		return "", fmt.Errorf("/nodes/enroll gave no credential")
	}
	return out.Credential, nil
}
