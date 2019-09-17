package main

import (
	"flag"
	"fmt"
	"log"
)

func main() {
	var (
		f = flag.String("f", "bin", "Filename")
		q = flag.Bool("q", false, "quite")
	)
	flag.Parse()
	fmt.Println(*f)

	emu, err := NewEmulator(1024*1024, 0x7c00, 0x7c00, *f)
	if err != nil {
		log.Fatal(err)
	}
	emu.Run(*q)
	emu.DumpEmulator()
}
