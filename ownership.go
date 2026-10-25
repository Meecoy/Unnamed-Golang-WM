package main

import (
	"fmt"
	"github.com/BurntSushi/xgb/xproto"
	"github.com/BurntSushi/xgbutil"
	"github.com/BurntSushi/xgbutil/ewmh"
	"github.com/BurntSushi/xgbutil/xevent"
	"github.com/BurntSushi/xgbutil/xprop"
	"github.com/BurntSushi/xgbutil/xwindow"
	"log"
	"time"
)

func getOwnership(X *xgbutil.XUtil, replace bool) error {
	otherWmRunning := false
	otherWmName := ""
	otherWmOwner := xproto.Window(xproto.WindowNone)

	xTime, err := currentTime(X)
	if err != nil {
		return err
	}

	selAtom, err := managerAtom(X)
	if err != nil {
		return err
	}

	reply, err := xproto.GetSelectionOwner(X.Conn(), selAtom).Reply()
	if err != nil {
		return err
	}

	if reply.Owner != xproto.WindowNone {
		otherWmRunning = true
		otherWmOwner = reply.Owner
		otherWmName, err = ewmh.GetEwmhWM(X)
		if err != nil || len(otherWmName) == 0 {
			log.Println("Couldn't determine current wm name")
			otherWmName = "Unknown"
		}
	}

	if otherWmRunning {
		if !replace {
			// TODO after adding flags
			return fmt.Errorf("...")
		} else {
			log.Printf("Waiting for %s to give transfer ownership to us...", otherWmName)
		}
	}

	log.Printf("Setting selection owner...")
	err = xproto.SetSelectionOwnerChecked(
		X.Conn(), X.Dummy(), selAtom, xTime).Check()
	if err != nil {
		return err
	}

	log.Printf("Getting selection owner...")
	reply, err = xproto.GetSelectionOwner(X.Conn(), selAtom).Reply()
	if err != nil {
		return err
	}
	if reply.Owner != X.Dummy() {
		return fmt.Errorf("Couldn't acquire ownership with SetSelectionOwner. "+
			"GetSelectionOwner claims that %d is the owner, but %d needs to be", reply.Owner, X.Dummy())
	}

	timeout := time.After(3 * time.Second)
	if otherWmRunning {
	OTHER_WM_SHUTDOWN:
		for {
			select {
			case <-timeout:
				return fmt.Errorf("FuturistWM failed to replace %s", otherWmName)
			default:
				log.Println("Polling for event")
				ev, err := X.Conn().PollForEvent()
				if err != nil {
					continue
				}
				log.Printf("Got event %s, error %s", ev, err)
				if destNotify, ok := ev.(xproto.DestroyNotifyEvent); ok {
					if destNotify.Window == otherWmOwner {
						break OTHER_WM_SHUTDOWN
					}
				}
				time.Sleep(100 * time.Millisecond)
			}
		}
	}

	log.Println("Window manager ownership received!")
	announce(X)

	xevent.SelectionClearFun(disown).Connect(X, X.Dummy())
	return nil
}

func disown(X *xgbutil.XUtil, ev xevent.SelectionClearEvent) {
	log.Println("Making space for another wm...")
	xevent.Quit(X)
}

func announce(X *xgbutil.XUtil) {
	typAtom, err := xprop.Atm(X, "MANAGER")
	if err != nil {
		log.Println(err)
		return
	}
	manSelAtom, err := managerAtom(X)
	if err != nil {
		log.Println(err)
		return
	}
	cm, err := xevent.NewClientMessage(
		32, X.RootWin(), typAtom, int(X.TimeGet()), int(manSelAtom), int(X.Dummy()))
	xproto.SendEvent(X.Conn(), false, X.RootWin(), xproto.EventMaskStructureNotify, string(cm.Bytes()))
}

func managerAtom(X *xgbutil.XUtil) (xproto.Atom, error) {
	return xprop.Atm(X, fmt.Sprintf("WM_S%d", X.Conn().DefaultScreen))
}

func currentTime(X *xgbutil.XUtil) (xproto.Timestamp, error) {
	wmClassAtom, err := xprop.Atm(X, "WM_CLASS")
	if err != nil {
		return 0, err
	}

	stringAtom, err := xprop.Atm(X, "STRING")

	err = xwindow.New(X, X.RootWin()).Listen(xproto.EventMaskPropertyChange)
	if err != nil {
		return 0, err
	}

	err = xproto.ChangePropertyChecked(X.Conn(), xproto.PropModeAppend, X.RootWin(), wmClassAtom, stringAtom, 8, 0, nil).Check()
	if err != nil {
		return 0, err
	}

	timeout := time.After(3 * time.Second)
	for {
		select {
		case <-timeout:
			return 0, fmt.Errorf("Expected a PropertyNotify event to get a valid timestamp, but received none")
		default:
			ev, err := X.Conn().PollForEvent()
			if err != nil {
				continue
			}

			if propNotify, ok := ev.(xproto.PropertyNotifyEvent); ok {
				X.TimeSet(propNotify.Time)
				return propNotify.Time, nil
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
}
