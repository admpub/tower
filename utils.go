package main

import (
	"errors"
	"net"
	"time"
)

func waitForServer(address string) (err error) {
	for {
		select {
		case <-time.After(1 * time.Second):
			_, err = net.Dial("tcp", address)
			if err == nil {
				return
			}
		case <-time.After(1 * time.Minute):
			return errors.New("Time out")
		}
	}
	return
}

func mustSuccess(err error) {
	if err != nil {
		panic(err)
	}
}
