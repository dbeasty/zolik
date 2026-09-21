// Command vapid-keys prints a fresh VAPID key pair for browser push, as the
// VAPID_PUBLIC_KEY and VAPID_PRIVATE_KEY lines the server reads.
//
// Generate once per deployment and keep it: every browser subscription is
// bound to the public key, so changing it silently unsubscribes everybody.
package main

import (
	"fmt"
	"log"

	"zolik/server/internal/notify"
)

func main() {
	pub, priv, err := notify.GenerateVAPIDKeys()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("VAPID_PUBLIC_KEY=%s\nVAPID_PRIVATE_KEY=%s\n", pub, priv)
}
