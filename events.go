package main

import (
	"github.com/BurntSushi/xgb/xproto"
	"github.com/BurntSushi/xgbutil"
	"github.com/BurntSushi/xgbutil/xevent"
	"log"
)

func (wm *WM) onMapRequest(X *xgbutil.XUtil, ev xevent.MapRequestEvent) {
	if client, ok := wm.Clients[ev.Window]; ok {
		xproto.MapWindow(X.Conn(), client.ID)
		xproto.MapWindow(X.Conn(), client.Frame)
		wm.focusClient(client, xproto.TimeCurrentTime)
		return
	}

	if err := wm.manage(ev.Window); err != nil {
		log.Printf("Cannot manage window %d: %v", ev.Window, err)
	}
}

func (wm *WM) onConfigureRequest(X *xgbutil.XUtil, ev xevent.ConfigureRequestEvent) {
	client, managed := wm.Clients[ev.Window]
	if !managed {
		var values []uint32
		if ev.ValueMask&xproto.ConfigWindowX != 0 {
			values = append(values, uint32(ev.X))
		}
		if ev.ValueMask&xproto.ConfigWindowY != 0 {
			values = append(values, uint32(ev.Y))
		}
		if ev.ValueMask&xproto.ConfigWindowWidth != 0 {
			values = append(values, uint32(ev.Width))
		}
		if ev.ValueMask&xproto.ConfigWindowHeight != 0 {
			values = append(values, uint32(ev.Height))
		}
		if ev.ValueMask&xproto.ConfigWindowBorderWidth != 0 {
			values = append(values, uint32(ev.BorderWidth))
		}
		if ev.ValueMask&xproto.ConfigWindowSibling != 0 {
			values = append(values, uint32(ev.Sibling))
		}
		if ev.ValueMask&xproto.ConfigWindowStackMode != 0 {
			values = append(values, uint32(ev.StackMode))
		}

		if err := xproto.ConfigureWindowChecked(
			X.Conn(),
			ev.Window,
			ev.ValueMask,
			values,
		).Check(); err != nil {
			log.Printf("Couldn't configure window %d: %#v ", ev.Window, err)
		}

		return
	}

	mask := ev.ValueMask

	if mask&xproto.ConfigWindowX != 0 {
		client.X = ev.X
	}
	if mask&xproto.ConfigWindowY != 0 {
		client.Y = ev.Y
	}
	if mask&xproto.ConfigWindowWidth != 0 {
		client.Width = ev.Width
	}
	if mask&xproto.ConfigWindowHeight != 0 {
		client.Height = ev.Height
	}

	var frameMask uint16
	var frameValues []uint32

	if mask&xproto.ConfigWindowX != 0 {
		frameMask |= xproto.ConfigWindowX
		frameValues = append(frameValues, uint32(client.X))
	}

	if mask&xproto.ConfigWindowY != 0 {
		frameMask |= xproto.ConfigWindowY
		frameValues = append(frameValues,
			uint32(client.Y-int16(TitleHeight)),
		)
	}

	if mask&xproto.ConfigWindowWidth != 0 {
		frameMask |= xproto.ConfigWindowWidth
		frameValues = append(frameValues,
			uint32(client.Width))
	}

	if mask&xproto.ConfigWindowHeight != 0 {
		frameMask |= xproto.ConfigWindowHeight
		frameValues = append(frameValues,
			uint32(client.Height+TitleHeight),
		)
	}

	if mask&xproto.ConfigWindowSibling != 0 {
		if sibling := wm.Clients[ev.Sibling]; sibling != nil {
			frameMask |= xproto.ConfigWindowSibling
			frameValues = append(
				frameValues,
				uint32(sibling.Frame),
			)
		}
	}

	if mask&xproto.ConfigWindowStackMode != 0 {
		frameMask |= xproto.ConfigWindowStackMode
		frameValues = append(
			frameValues,
			uint32(ev.StackMode),
		)
	}

	if frameMask != 0 {
		if err := xproto.ConfigureWindowChecked(X.Conn(), client.Frame, frameMask, frameValues).Check(); err != nil {
			log.Printf("Couldn't configure frame %d for window %d: %#v", client.Frame, ev.Window, err)
		}
	}

	var childMask uint16
	var childValues []uint32

	if mask&xproto.ConfigWindowWidth != 0 {
		childMask |= xproto.ConfigWindowWidth
		childValues = append(childValues, uint32(client.Width))
	}
	if mask&xproto.ConfigWindowHeight != 0 {
		childMask |= xproto.ConfigWindowHeight
		childValues = append(childValues, uint32(client.Height))
	}
	if childMask != 0 {
		if err := xproto.ConfigureWindowChecked(X.Conn(), client.ID, childMask, childValues).Check(); err != nil {
			log.Printf("Couldn't resize window %d: %#v", client.ID, err)
		}
	}

	notify := xproto.ConfigureNotifyEvent{
		Event:        client.ID,
		Window:       client.ID,
		AboveSibling: xproto.WindowNone,
		X:            client.X,
		Y:            client.Y,
		Width:        client.Width,
		Height:       client.Height,
		BorderWidth:  0,
	}

	xproto.SendEvent(
		X.Conn(),
		false,
		client.ID,
		xproto.EventMaskStructureNotify,
		string(notify.Bytes()),
	)

	if mask&(xproto.ConfigWindowWidth|xproto.ConfigWindowHeight) != 0 {
		wm.drawTitle(client)
	}
}

func (wm *WM) onExpose(X *xgbutil.XUtil, ev xevent.ExposeEvent) {
	if ev.Count != 0 {
		return
	}

	if client := wm.ByFrame[ev.Window]; client != nil {
		wm.drawTitle(client)
	}
}

func (wm *WM) onPropertyNotify(X *xgbutil.XUtil, ev xevent.PropertyNotifyEvent) {
	client := wm.Clients[ev.Window]

	if client == nil || (ev.Atom != wm.wmName && ev.Atom != wm.netWmName) {
		return
	}

	client.Title = wm.getTitle(client.ID)
	wm.drawTitle(client)
}

func (wm *WM) onDestroyNotify(X *xgbutil.XUtil, ev xevent.DestroyNotifyEvent) {
	client := wm.Clients[ev.Window]
	if client == nil{
		return
	}

	log.Printf("DestroyNotify: window %d frame %d event %d", ev.Window, client.Frame, ev.Event)

	wm.unmanage(client, false)
}

func (wm *WM) onUnmapNotify(X *xgbutil.XUtil, ev xevent.UnmapNotifyEvent) {
	client := wm.Clients[ev.Window]
	if client == nil {
		return
	}

	log.Printf("UnmapNotify: window %d frame %d event %d", ev.Window, client.Frame, ev.Event)

	if err := xproto.UnmapWindowChecked(X.Conn(), client.Frame).Check(); err != nil{
		log.Printf("Couldn't unmap frame: %#v", err)
	}
}
