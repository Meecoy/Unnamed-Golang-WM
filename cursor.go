package main

import (
	"github.com/BurntSushi/xgb/xproto"
	"github.com/BurntSushi/xgbutil"
	"github.com/BurntSushi/xgbutil/xcursor"
)

func setRootCursor(X *xgbutil.XUtil, cursorNum uint16) error {
	cursor, err := xcursor.CreateCursor(X, cursorNum)
	if err != nil {
		return err
	}

	return xproto.ChangeWindowAttributesChecked(
		X.Conn(),X.RootWin(), xproto.CwCursor, []uint32{uint32(cursor)},
	).Check()
}
