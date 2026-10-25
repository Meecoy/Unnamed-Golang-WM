package main

import (
	"github.com/BurntSushi/xgb/xproto"
	"github.com/BurntSushi/xgbutil"
	"github.com/BurntSushi/xgbutil/xcursor"
	"github.com/BurntSushi/xgbutil/xevent"
	"log"
)

type DragState struct {
	Client *Client

	StartMouseX int16
	StartMouseY int16

	StartX int16
	StartY int16
}

func (wm *WM) onDragStart(X *xgbutil.XUtil, ev xevent.ButtonPressEvent) {
	if ev.Detail != xproto.ButtonIndex1 {
		return
	}

	if ev.EventY < 0 || ev.EventY >= int16(TitleHeight) {
		return
	}

	client := wm.ByFrame[ev.Event]
	if client == nil || wm.drag != nil {
		return
	}

	wm.focusClient(client, ev.Time)

	reply, err := xproto.GrabPointer(
		X.Conn(),
		false,
		X.RootWin(),
		xproto.EventMaskPointerMotion|xproto.EventMaskButtonRelease,
		xproto.GrabModeAsync,
		xproto.GrabModeAsync,
		xproto.WindowNone,
		xproto.CursorNone,
		ev.Time,
	).Reply()

	setRootCursor(X, xcursor.Fleur)

	if err != nil {
		log.Printf("Couldn't grab pointer: %#v", err)
		return
	}

	if reply.Status != xproto.GrabStatusSuccess {
		log.Println("Grab pointer operation unsuccessful")
		return
	}

	wm.drag = &DragState{
		Client:      client,
		StartMouseX: ev.RootX,
		StartMouseY: ev.RootY,
		StartX:      client.X,
		StartY:      client.Y,
	}
}

func (wm *WM) onDragMotion(X *xgbutil.XUtil, ev xevent.MotionNotifyEvent) {
	if wm.drag == nil {
		return
	}

	drag := wm.drag

	dx := int32(ev.RootX) - int32(drag.StartMouseX)
	dy := int32(ev.RootY) - int32(drag.StartMouseY)

	newX := int16(int32(drag.StartX) + dx)
	newY := int16(int32(drag.StartY) + dy)

	xproto.ConfigureWindow(X.Conn(), drag.Client.Frame, xproto.ConfigWindowX|xproto.ConfigWindowY, []uint32{
		uint32(newX), uint32(newY - int16(TitleHeight)),
	},
	)

	drag.Client.X = newX
	drag.Client.Y = newY
}

func (wm *WM) onDragEnd(X *xgbutil.XUtil, ev xevent.ButtonReleaseEvent) {
	if wm.drag == nil || ev.Detail != xproto.ButtonIndex1 {
		return
	}

	wm.drag = nil

	xproto.UngrabPointer(
		X.Conn(),
		ev.Time,
	)

	setRootCursor(X, xcursor.LeftPtr)
}
