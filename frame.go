package main

import (
	"fmt"
	"log"

	"github.com/BurntSushi/xgb/xproto"
	"github.com/BurntSushi/xgbutil/ewmh"
	"github.com/BurntSushi/xgbutil/icccm"
	"github.com/BurntSushi/xgbutil/xevent"
)

func (wm *WM) initTitleFont() error {
	conn := wm.X.Conn()

	font, err := xproto.NewFontId(conn)
	if err != nil {
		return err
	}

	if err := xproto.OpenFontChecked(
		conn, font, uint16(len("fixed")), "fixed",
	).Check(); err != nil {
		return err
	}

	wm.font = font

	gc, err := xproto.NewGcontextId(conn)
	if err != nil {
		return err
	}

	if err := xproto.CreateGCChecked(
		conn,
		gc,
		xproto.Drawable(wm.X.RootWin()),
		xproto.GcForeground|xproto.GcBackground|xproto.GcFont,
		[]uint32{TitleFG, TitleBG, uint32(font)},
	).Check(); err != nil {
		return err
	}

	wm.gc = gc

	return nil
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

	geom, err := xproto.GetGeometry(
		conn,
		xproto.Drawable(id),
	).Reply()
	if err != nil {
		return err
	}

	screen := wm.X.Screen()

	screenW := int32(screen.WidthInPixels)
	screenH := int32(screen.HeightInPixels)

	const margin int32 = 20
	titleH := int32(TitleHeight)

	width := int32(geom.Width)
	height := int32(geom.Height)

	log.Printf(
		"Initial geometry: window=%d x=%d y=%d width=%d height=%d screen=%dx%d",
		id,
		geom.X,
		geom.Y,
		geom.Width,
		geom.Height,
		screenW,
		screenH,
	)

	maxW := max(1, screenW-2*margin)
	maxH := max(1, screenH-2*margin-titleH)

	width = min(max(width, 1), maxW)
	height = min(max(height, 1), maxH)

	frameX := (screenW - width) / 2
	frameY := (screenH - height - titleH) / 2

	frameX = max(0, frameX)
	frameY = max(0, frameY)

	x := int16(frameX)
	y := int16(frameY + titleH)

	frame, err := xproto.NewWindowId(conn)
	if err != nil {
		return err
	}

	mask := uint32(
		xproto.CwBackPixmap |
			xproto.CwBorderPixel|
			xproto.CwOverrideRedirect |
			xproto.CwEventMask,
	)

	events := uint32(
		xproto.EventMaskExposure |
			xproto.EventMaskSubstructureRedirect |
			xproto.EventMaskSubstructureNotify |
			xproto.EventMaskButtonPress,
	)

	err = xproto.CreateWindowChecked(
		conn,
		screen.RootDepth,
		frame,
		wm.X.RootWin(),
		int16(frameX),
		int16(frameY),
		uint16(width),
		uint16(height+titleH),
		1,
		xproto.WindowClassInputOutput,
		screen.RootVisual,
		mask,
		[]uint32{
			xproto.BackPixmapNone,
			TitleBG,
			1,
			events,
		},
	).Check()

	if err != nil {
		return err
	}

	if err = xproto.ChangeSaveSetChecked(
		conn,
		xproto.SetModeInsert,
		id,
	).Check(); err != nil {
		xproto.DestroyWindow(conn, frame)
		return err
	}

	if err = xproto.ConfigureWindowChecked(
		conn,
		id,
		xproto.ConfigWindowBorderWidth,
		[]uint32{0},
	).Check(); err != nil {
		xproto.DestroyWindow(conn, frame)
		return err
	}

	if err = xproto.ReparentWindowChecked(
		conn,
		id,
		frame,
		0,
		int16(TitleHeight),
	).Check(); err != nil {
		xproto.DestroyWindow(conn, frame)
		return err
	}

	if err = xproto.ConfigureWindowChecked(
		conn,
		id,
		xproto.ConfigWindowWidth|xproto.ConfigWindowHeight,
		[]uint32{
			uint32(width),
			uint32(height),
		},
	).Check(); err != nil {
		xproto.DestroyWindow(conn, frame)
		return err
	}

	client := &Client{
		ID:     id,
		Frame:  frame,
		X:      x,
		Y:      y,
		Width:  uint16(width),
		Height: uint16(height),
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

	log.Printf(
		"Managed window %d (%q) at (%d, %d), size %dx%d",
		id,
		client.Title,
		client.X,
		client.Y,
		client.Width,
		client.Height,
	)

	return nil
}

func (wm *WM) unmanage(client *Client, restore bool) {
	conn := wm.X.Conn()

	if wm.drag != nil && wm.drag.Client == client {
		wm.drag = nil
		xproto.UngrabPointer(conn, xproto.TimeCurrentTime)
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
			log.Printf(
				"Cannot restore window %d: %v",
				client.ID,
				err,
			)
		}

		xproto.ChangeSaveSet(
			conn,
			xproto.SetModeDelete,
			client.ID,
		)
	}

	if err := xproto.DestroyWindowChecked(
		conn,
		client.Frame,
	).Check(); err != nil {
		log.Printf(
			"Cannot destroy frame %d: %v",
			client.Frame,
			err,
		)
	}
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

		if r >= 32 && r <= 255 {
			text = append(text, byte(r))
		} else {
			text = append(text, '?')
		}
	}

	if len(text) == 0 {
		return
	}

	textY := int16(TitleHeight) - 4

	if err := xproto.ImageText8Checked(
		conn,
		byte(len(text)),
		xproto.Drawable(client.Frame),
		wm.gc,
		9,
		textY,
		string(text),
	).Check(); err != nil {
		log.Printf(
			"Cannot draw title for window %d: %v",
			client.ID,
			err,
		)
	}
}
