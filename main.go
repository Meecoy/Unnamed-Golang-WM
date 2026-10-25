package main

import (
	"log"
	//	"github.com/BurntSushi/xgb"
	"github.com/BurntSushi/xgbutil"
	"github.com/BurntSushi/xgbutil/xcursor"
)

func main() {
	X, err := xgbutil.NewConn()
	if err != nil {
		log.Fatalln(err)
	}
	defer X.Conn().Close()
	// TODO change replace bool to flag after implementing flags
	if err := getOwnership(X, true); err != nil {
		log.Fatalf("Couldn't establish window manager ownership (%#v), quitting...", err)
	}

	if err := setRootCursor(X, xcursor.LeftPtr); err != nil {
		log.Printf("Couldn't set root cursor: %#v", err)
	}

	wm := NewWM(X)

	if err := wm.Run(); err != nil {
		log.Fatalf("Couldn't start main event loop: %#v", err)
	}
}
