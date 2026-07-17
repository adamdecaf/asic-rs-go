// Example: scan a subnet for miners.
//
//	ASIC_SUBNET=192.168.1.0/24 go run ./examples/scan
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/adamdecaf/asic-rs-go/asicrs"
)

func main() {
	subnet := os.Getenv("ASIC_SUBNET")
	if subnet == "" {
		subnet = "192.168.68.0/24"
		fmt.Fprintf(os.Stderr, "ASIC_SUBNET not set; scanning %s\n", subnet)
	}

	factory, err := asicrs.NewFactoryFromSubnet(subnet)
	if err != nil {
		log.Fatal(err)
	}
	defer factory.Close()

	factory.SetConcurrentLimit(500)
	factory.SetIdentificationTimeoutSecs(5)
	factory.SetAdaptiveConcurrency()

	fmt.Printf("scanning %d hosts in %s ...\n", factory.Len(), subnet)

	miners, err := factory.Scan()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("found %d miner(s)\n", len(miners))
	for _, m := range miners {
		summary, _ := m.Summary()
		fmt.Println(" -", summary)
		m.Close()
	}
}
