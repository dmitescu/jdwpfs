package main

import (
	"log"
	
	jdwp "github.com/omerye/gojdb/jdwp"
)

func JdwpfsPluginEntrypoint(name string, event jdwp.Event) error {
	log.Printf("eeey %s ran!\n", name)
	return nil
}
