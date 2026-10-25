package main

import (
	"github.com/BurntSushi/xgb/xproto"
	"github.com/BurntSushi/xgbutil"
	"github.com/BurntSushi/xgbutil/xevent"
	"github.com/BurntSushi/xgbutil/xprop"
	"github.com/BurntSushi/xgbutil/xwindow"
)

const (
	TitleHeight = uint16(16)
	TitleBG     = uint32(0x41b011)
	TitleFG     = uint32(0xffffff)
)

type WM struct {
	X         *xgbutil.XUtil
	Clients   map[xproto.Window]*Client
	ByFrame   map[xproto.Window]*Client
	font      xproto.Font
	gc        xproto.Gcontext
	wmName    xproto.Atom
	netWmName xproto.Atom
	drag      *DragState
}

type Client struct {
	ID xproto.Window
	Frame xproto.Window
	X,Y int16
	Width, Height uint16
	Title string
}

func NewWM(X *xgbutil.XUtil) *WM {
	return &WM{
		X:       X,
		Clients: make(map[xproto.Window]*Client),
		ByFrame: make(map[xproto.Window]*Client),
	}
}

func (wm *WM) Run() error {
	if err := wm.initTitleFont(); err != nil{
		return err
	}
	defer wm.closeTitleFont()

	var err error

	wm.wmName, err = xprop.Atm(wm.X, "WM_NAME")
	if err != nil{
		return err
	}
	wm.netWmName, err = xprop.Atm(wm.X, "_NET_WM_NAME")
	if err != nil{
		return err
	}

	root := wm.X.RootWin()

	xevent.MotionNotifyFun(wm.onDragMotion).Connect(wm.X, root)
	xevent.ButtonReleaseFun(wm.onDragEnd).Connect(wm.X, root)
	xevent.MapRequestFun(wm.onMapRequest).Connect(wm.X, root)
	xevent.ConfigureRequestFun(wm.onConfigureRequest).Connect(wm.X, root)

	if err := xwindow.New(wm.X, root).Listen(
		xproto.EventMaskSubstructureRedirect |
			xproto.EventMaskSubstructureNotify,
	); err != nil{
		return err
	}

	xevent.Main(wm.X)
	return nil
}
