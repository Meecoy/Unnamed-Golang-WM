package main

import (
	"fmt"
	"github.com/BurntSushi/xgb/xproto"
	"github.com/BurntSushi/xgbutil/ewmh"
	"github.com/BurntSushi/xgbutil/icccm"
	"github.com/BurntSushi/xgbutil/xevent"
	"log"
)

func (wm *WM) initTitleFont() error {
	conn := wm.X.Conn()

	font, err := xproto.NewFontId(conn)
	if err != nil {
		return err
	}

	if err := xproto.OpenFontChecked(conn, font, uint16(len("fixed")), "fixed").Check(); err != nil {
		return err
	}

	wm.font = font

	gc, err := xproto.NewGcontextId(conn)
	if err != nil {
		return err
	}

	wm.gc = gc

	return xproto.CreateGCChecked(conn, gc, xproto.Drawable(wm.X.RootWin()), xproto.GcForeground|xproto.GcBackground|xproto.GcFont, []uint32{TitleFG, TitleBG, uint32(font)}).Check()
}

func (wm *WM) closeTitleFont() {
	if wm.gc != 0 {
		xproto.FreeGC(wm.X.Conn(), wm.gc)
	}

	if wm.font != 0 {
		xproto.CloseFont(wm.X.Conn(), wm.font)
	}
}

func (wm *WM) getTitle(id xproto.Window) string {
	name, err := ewmh.WmNameGet(wm.X, id)

	if err != nil || name == "" {
		name, err = icccm.WmNameGet(wm.X, id)
	}

	if err != nil || name == "" {
		return fmt.Sprintf("Window %d", id)
	}

	return name
}

func (wm *WM) manage(id xproto.Window) error {
	conn := wm.X.Conn()

	if _, ok := wm.Clients[id]; ok {
		return nil
	}

	attrs, err := xproto.GetWindowAttributes(conn, id).Reply()
	if err != nil {
		return err
	}

	if attrs.OverrideRedirect {
		return xproto.MapWindowChecked(conn, id).Check()
	}

	geom, err := xproto.GetGeometry(conn, xproto.Drawable(id)).Reply()
	if err != nil {
		return err
	}

	frame, err := xproto.NewWindowId(conn)
	if err != nil {
		return err
	}

	mask := uint32(xproto.CwBackPixmap | xproto.CwOverrideRedirect | xproto.CwEventMask)
	events := uint32(xproto.EventMaskExposure | xproto.EventMaskSubstructureRedirect | xproto.EventMaskSubstructureNotify | xproto.EventMaskButtonPress)
	err = xproto.CreateWindowChecked(conn, wm.X.Screen().RootDepth, frame, wm.X.RootWin(), geom.X, geom.Y-int16(TitleHeight), geom.Width, geom.Height+TitleHeight, 0, xproto.WindowClassInputOutput, wm.X.Screen().RootVisual, mask, []uint32{xproto.BackPixmapNone, 1, events}).Check()
	if err != nil {
		return err
	}

	if err = xproto.ChangeSaveSetChecked(conn, xproto.SetModeInsert, id).Check(); err != nil {
		xproto.DestroyWindow(conn, frame)
		return err
	}

	if err = xproto.ConfigureWindowChecked(conn, id, xproto.ConfigWindowBorderWidth, []uint32{0}).Check(); err != nil {
		xproto.DestroyWindow(conn, frame)
		return err
	}

	if err = xproto.ReparentWindowChecked(conn, id, frame, 0, int16(TitleHeight)).Check(); err != nil {
		xproto.DestroyWindow(conn, frame)
		return err
	}

	client := &Client{
		ID:     id,
		Frame:  frame,
		X:      geom.X,
		Y:      geom.Y,
		Width:  geom.Width,
		Height: geom.Height,
		Title:  wm.getTitle(id),
	}

	wm.Clients[id] = client
	wm.ByFrame[frame] = client

	xevent.MapRequestFun(wm.onMapRequest).Connect(wm.X, frame)
	xevent.ConfigureRequestFun(wm.onConfigureRequest).Connect(wm.X, frame)
	xevent.ExposeFun(wm.onExpose).Connect(wm.X, frame)
	xevent.ButtonPressFun(wm.onDragStart).Connect(wm.X, frame)
	xevent.PropertyNotifyFun(wm.onPropertyNotify).Connect(wm.X, id)
	xevent.DestroyNotifyFun(wm.onDestroyNotify).Connect(wm.X, id)
	xevent.UnmapNotifyFun(wm.onUnmapNotify).Connect(wm.X, id)

	if err = xproto.ChangeWindowAttributesChecked(
		conn,
		id,
		xproto.CwEventMask,
		[]uint32{
			uint32(
				xproto.EventMaskStructureNotify |
					xproto.EventMaskPropertyChange,
			),
		},
	).Check(); err != nil {
		wm.unmanage(client, true)
		return err
	}
	if err = xproto.MapWindowChecked(conn, id).Check(); err != nil {
		return err
	}

	if err = xproto.MapWindowChecked(conn, frame).Check(); err != nil {
		return err
	}

	wm.drawTitle(client)

	log.Printf("Managed window %d (%q)", id, client.Title)

	return nil
}

func (wm *WM) unmanage(client *Client, restore bool) {
	conn := wm.X.Conn()
	
	if wm.drag != nil && wm.drag.Client == client {
		wm.drag = nil
		xproto.UngrabPointer(
			conn,
			xproto.TimeCurrentTime,
		)
	}

	delete(wm.Clients, client.ID)
	delete(wm.ByFrame, client.Frame)

	xevent.Detach(wm.X, client.ID)
	xevent.Detach(wm.X, client.Frame)
	
	if restore {
		if err := xproto.ReparentWindowChecked(
			conn,
			client.ID,
			wm.X.RootWin(),
			client.X,
			client.Y,
		).Check(); err != nil {
			log.Printf("Cannot restore window %d: %v", client.ID, err)
		}
		
		xproto.ChangeSaveSet(
			conn,
			xproto.SetModeDelete,
			client.ID,
		)
	}

	if err := xproto.DestroyWindowChecked(conn, client.Frame).Check(); err != nil{
		log.Printf("Couldn't destroy frame: %#v", err)
	}

	log.Printf("Unmanaged window %d", client.ID)
}

func (wm *WM) drawTitle(client *Client) {
	conn := wm.X.Conn()

	xproto.ChangeGC(
		conn,
		wm.gc,
		xproto.GcForeground,
		[]uint32{TitleBG},
	)

	xproto.PolyFillRectangle(
		conn,
		xproto.Drawable(client.Frame),
		wm.gc,
		[]xproto.Rectangle{
			{
				X:      0,
				Y:      0,
				Width:  client.Width,
				Height: TitleHeight,
			},
		},
	)

	xproto.ChangeGC(
		conn,
		wm.gc,
		xproto.GcForeground,
		[]uint32{TitleFG},
	)

	var text []byte

	for _, r := range client.Title {
		if len(text) >= 250 {
			break
		}

		if r > 31 && r < 256 {
			text = append(text, byte(r))
		} else {
			text = append(text, '?')
		}
	}

	if len(text) == 0 {
		return
	}

	if err := xproto.ImageText8(
		conn,
		byte(len(text)),
		xproto.Drawable(client.Frame),
		wm.gc,
		int16(9),
		int16(12),
		string(text),
	).Check(); err != nil {
		log.Printf("Couldn't draw %d title: %#v", client.Frame, err)
	}
}
