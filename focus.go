package main

import(
	"log"
	"github.com/BurntSushi/xgb/xproto"
	"github.com/BurntSushi/xgbutil"
	"github.com/BurntSushi/xgbutil/xevent"
)

func (wm *WM) focusClient(client *Client, time xproto.Timestamp){
	if client == nil{
		return
	}

	err := xproto.SetInputFocusChecked(
		wm.X.Conn(),
		xproto.InputFocusPointerRoot,
		client.ID,
		time,
	).Check()
	if err != nil{
		log.Printf("Couldn't focus window %d: %#v", client.ID, err)
	}
}

// Connect() method uses XUtil so we have to add it anyway :(

func (wm *WM) onClientEnter(X *xgbutil.XUtil, ev xevent.EnterNotifyEvent){
	if ev.Mode != xproto.NotifyModeNormal {
		return
	}

	client := wm.Clients[ev.Event]
	wm.focusClient(client, ev.Time)
}
