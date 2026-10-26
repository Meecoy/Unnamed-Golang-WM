package main

import(
	"log"
	"os/exec"

	"github.com/BurntSushi/xgbutil"
	"github.com/BurntSushi/xgbutil/keybind"
	"github.com/BurntSushi/xgbutil/xevent"
)

type Keybind struct {
	Key string
	Action func()
}

func (wm *WM) initKeybinds() error {
	keybind.Initialize(wm.X)

	bindings, err := wm.parseKeybinds()
	if err != nil{
		return err
	}

	for _, binding := range bindings {

		err := keybind.KeyPressFun(
			func(X *xgbutil.XUtil, ev xevent.KeyPressEvent){
				binding.Action()
			},
		).Connect(wm.X, wm.X.RootWin(), binding.Key, true)

		if err != nil{
			return err
		}
	}

	return nil
}

func (wm *WM) spawn(name string, args ...string){
	cmd := exec.Command(name, args...)

	if err := cmd.Start(); err != nil {
		log.Printf("Couldn't start %s: %#v", name, err)
		return
	}

	go func(){
		if err := cmd.Wait(); err != nil{
			log.Printf("Exec %s exited: %v", name, err)
		}
	}()
}
