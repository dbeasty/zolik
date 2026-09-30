// Command tunnelpipe runs an embedded host and one Bluetooth tunnel over
// stdin and stdout, so the app's guest side (src/net/ble) can be tested
// end to end against the real Go host without a radio.
//
// Frames both ways are a 4-byte big-endian length and then one tunnel
// message: exactly what the native code puts on the air, before chunking.
//
//	go build -o /tmp/tunnelpipe ./mobile/tunnelpipe
//	ZOLIK_TUNNEL_PIPE=/tmp/tunnelpipe npx jest src/net/ble/pipe
package main

import (
	"bufio"
	"encoding/binary"
	"io"
	"log"
	"os"
	"sync"

	"zolik/server/mobile/zolikcore"
)

type stdoutSink struct {
	mu sync.Mutex
	w  *bufio.Writer
}

func (s *stdoutSink) Send(msg []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var n [4]byte
	binary.BigEndian.PutUint32(n[:], uint32(len(msg)))
	_, _ = s.w.Write(n[:])
	_, _ = s.w.Write(msg)
	_ = s.w.Flush()
}

func main() {
	log.SetOutput(os.Stderr)
	dir, err := os.MkdirTemp("", "tunnelpipe")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(dir)
	h, err := zolikcore.Start(dir)
	if err != nil {
		log.Fatal(err)
	}
	defer h.Stop()

	tun := h.NewTunnel(&stdoutSink{w: bufio.NewWriter(os.Stdout)}, "pipe")
	in := bufio.NewReader(os.Stdin)
	for {
		var n [4]byte
		if _, err := io.ReadFull(in, n[:]); err != nil {
			return
		}
		msg := make([]byte, binary.BigEndian.Uint32(n[:]))
		if _, err := io.ReadFull(in, msg); err != nil {
			return
		}
		tun.Receive(msg)
	}
}
